# Phase 04 — SSE Hub + Live Stream Endpoint

## Context
- Depends on: phase-03-stream-json-parser-and-guards
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md

## Overview
**Priority:** P0
**Status:** pending
**Effort:** S

In-memory SSE hub + `GET /api/jobs/:id/stream` endpoint. Replays historical `job_logs` first (with `Last-Event-ID` resume), then subscribes to the hub for live lines from running jobs. Worker's `LogWriter` now fans out to the hub on each append.

## Key insights
- SSE (text/event-stream) is a one-way server→client stream over plain HTTP — zero WebSocket complexity.
- Browser `EventSource` auto-reconnects and sends `Last-Event-ID` header on reconnect — we give each event the `job_log.seq` as its ID so resume is exact.
- Caddy reverse proxy buffers by default → must set `flush_interval -1` (documented in phase 07, verified here with direct curl).
- Non-blocking fan-out: slow subscribers must not back up the worker. Drop events if subscriber buffer is full rather than block.

## Requirements
**Functional**
- `GET /api/jobs/:id/stream` authenticated (JWT via cookie or `?token=` query).
- Replays all existing `job_logs` rows for the job (filtered by `seq > lastEventID` if provided) before any live lines.
- If job is still running/pending, subscribes to hub and streams new lines until job finishes, then closes.
- If job is already terminal (done/failed/cancelled), sends all historical rows then closes.
- Multi-subscriber: two clients can watch the same running job concurrently.

**Non-functional**
- Buffered subscriber channel size 256; full → drop event and log warning.
- Client disconnect detected via request context → subscriber cleaned up.
- No goroutine leak on disconnect (verified by restart + curl tests).

## Architecture
```
internal/sse/
  hub.go            # Hub struct, Subscribe, Unsubscribe, Publish

internal/handler/
  stream_handler.go # GET /api/jobs/:id/stream
```

Worker wiring:
- `LogWriter.Append` calls `hub.Publish(jobID, seq, line)` synchronously after in-memory buf push.
- Pool takes `SSEPublisher` interface in constructor (avoid import cycles).

## Related code files
**Create:**
- `internal/sse/hub.go`
- `internal/handler/stream_handler.go`

**Modify:**
- `internal/worker/log_writer.go` — call hub on Append
- `internal/worker/pool.go` — accept `SSEPublisher` interface
- `internal/router/router.go` — wire new route
- `cmd/server/main.go` — construct hub, inject into pool + handler

## Implementation steps
1. `internal/sse/hub.go`:
   ```go
   type Event struct { Seq int; Line string }

   type Hub struct {
     mu   sync.RWMutex
     subs map[string]map[chan Event]struct{}
   }

   func NewHub() *Hub

   func (h *Hub) Subscribe(jobID string) (<-chan Event, func()) {
     ch := make(chan Event, 256)
     h.mu.Lock()
     if h.subs[jobID] == nil { h.subs[jobID] = make(map[chan Event]struct{}) }
     h.subs[jobID][ch] = struct{}{}
     h.mu.Unlock()
     unsub := func() {
       h.mu.Lock()
       delete(h.subs[jobID], ch)
       if len(h.subs[jobID]) == 0 { delete(h.subs, jobID) }
       h.mu.Unlock()
       close(ch)
     }
     return ch, unsub
   }

   func (h *Hub) Publish(jobID string, seq int, line string) {
     h.mu.RLock()
     defer h.mu.RUnlock()
     for ch := range h.subs[jobID] {
       select {
       case ch <- Event{Seq: seq, Line: line}:
       default:
         // drop: slow consumer
       }
     }
   }
   ```
2. `internal/handler/stream_handler.go`:
   ```go
   func (h *StreamHandler) Stream(c *gin.Context) {
     jobID := c.Param("id")
     job, err := h.jobRepo.FindByID(jobID)
     if err != nil { c.JSON(404, ...); return }

     lastID := parseLastEventID(c) // header or query

     c.Writer.Header().Set("Content-Type", "text/event-stream")
     c.Writer.Header().Set("Cache-Control", "no-cache")
     c.Writer.Header().Set("Connection", "keep-alive")
     c.Writer.Header().Set("X-Accel-Buffering", "no")
     flusher, _ := c.Writer.(http.Flusher)

     // Replay history
     logs, _ := h.logRepo.ListAfterSeq(jobID, lastID)
     for _, lg := range logs {
       writeSSE(c.Writer, lg.Seq, lg.Line)
       flusher.Flush()
     }

     if job.Status != "pending" && job.Status != "running" {
       return // terminal, close
     }

     ch, unsub := h.hub.Subscribe(jobID)
     defer unsub()

     ctx := c.Request.Context()
     for {
       select {
       case <-ctx.Done():
         return
       case ev, ok := <-ch:
         if !ok { return }
         writeSSE(c.Writer, ev.Seq, ev.Line)
         flusher.Flush()
         // After each event, cheap poll: if job now terminal, end stream
         if ev.Seq % 20 == 0 {
           if cur, _ := h.jobRepo.FindByID(jobID); cur != nil && cur.Status != "running" && cur.Status != "pending" {
             return
           }
         }
       }
     }
   }

   func writeSSE(w io.Writer, seq int, data string) {
     fmt.Fprintf(w, "id: %d\ndata: %s\n\n", seq, escapeNewlines(data))
   }
   ```
3. `parseLastEventID`: check header `Last-Event-ID` first, else query `?lastEventId=`, default 0.
4. `job_log_repository.ListAfterSeq(jobID, afterSeq) ([]JobLog, error)` — `WHERE job_id=? AND seq>? ORDER BY seq`.
5. `log_writer.go` — after appending to in-mem buf, `hub.Publish(jobID, seq, line)`. Define `type SSEPublisher interface { Publish(jobID string, seq int, line string) }` in `internal/worker` and have hub implement it (no cycle since worker only imports interface).
6. Router: `GET /api/jobs/:id/stream` → stream handler, JWT middleware applied (allow token in query for EventSource, since it can't set headers — or use session cookie from NextAuth).
7. Caddy config note: add `flush_interval -1` in the `/api/*` reverse_proxy block — documented + verified in phase 07.

## Todo list
- [x] hub.go pub/sub with drop-on-full
- [x] stream_handler.go replay + live + terminal detection
- [x] log_writer wired to hub via interface
- [x] job_log_repository.ListAfterSeq
- [x] Accept JWT via `?token=` query param for EventSource compatibility
- [x] Manual test: curl the SSE endpoint against a running job, see lines in real-time
- [x] Manual test: close + reconnect mid-stream with Last-Event-ID → resumes from next seq
- [x] Manual test: two curl subscribers on same job → both receive every line
- [x] Manual test: subscribe to already-terminal job → receives full history then closes

## Success criteria
1. `curl -N -H "Authorization: Bearer $JWT" http://localhost:8080/api/jobs/$ID/stream` streams lines live during execution, closes cleanly on done/failed/cancelled.
2. Mid-stream kill + reconnect with `-H "Last-Event-ID: 42"` → resumes with seq 43 onward.
3. Two concurrent curl subscribers → identical output.
4. Server restart during stream → client receives connection close; on reconnect, replays history from last seen seq (since running→failed during recovery).
5. `pprof goroutine` before/after stream connect/disconnect → no leak.

## Risks
| Risk | Mitigation |
|---|---|
| EventSource can't set `Authorization` header | Accept `?token=` query param; NextAuth can include it when opening EventSource |
| Publisher blocked by slow subscriber | Non-blocking select with default drop — documented, acceptable |
| Stream never closes if job stuck in `running` after crash | Crash recovery in phase 02 flips running→failed on boot; new subs will see terminal state |
| Gin writer not flushing to Caddy | `X-Accel-Buffering: no` + phase 07 Caddy `flush_interval -1` |
| Dropping lines hides failures from UI | Phase 06 UI shows “history may be incomplete” banner if gaps in seq detected |

## Security
- JWT validated before any stream output. Invalid/missing → 401.
- Query-param token: same validation path, but warn in README that URLs with tokens may leak into logs — consider short-lived token or rely on cookie.
- Hub never persists state; nothing leaks across processes.

## Next steps
Phase 05 — frontend scaffold so phase 06 can consume this endpoint.
