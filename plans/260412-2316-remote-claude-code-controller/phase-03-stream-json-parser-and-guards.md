# Phase 03 — stream-json Parser + JobLog + Cost/Timeout Guards

## Context
- Depends on: phase-02-worker-pool-subprocess-runner
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md

## Overview
**Priority:** P0
**Status:** pending
**Effort:** M

Replace the dumb `io.ReadAll` stdout capture from phase 02 with a line-by-line scanner that parses Claude Code's `stream-json` events, persists each event to `job_logs`, extracts summary fields from the final `result` event, and enforces the per-job cost cap by monitoring `total_cost_usd` on the terminal event (cancelling if exceeded).

## Key insights
- `stream-json` emits one compact JSON per line. Event types seen: `system`, `assistant`, `user`, `result`.
- The `result` event is the final one and contains: `result` (text), `is_error`, `duration_ms`, `duration_api_ms`, `total_cost_usd`, `num_turns`, `session_id`, `stop_reason`, `usage`.
- Cost is only known at the end of the event stream — there's no mid-stream partial cost. Cost guard is therefore enforced via (a) `result.total_cost_usd > cap` → mark failed post-hoc, OR (b) hard wall-clock timeout (`JobTimeoutMin`) + a token-count-based estimator stretch goal.
- Very long lines possible (assistant messages with big code blocks) — must set `bufio.Scanner.Buffer` above the 64 KB default. Use 4 MB.
- stderr also captured into job_logs with `stream='stderr'` for debuggability.

## Requirements
**Functional**
- Each line of subprocess stdout → one `job_logs` row with monotonic `seq`.
- stderr lines → same table with `stream='stderr'`.
- On `type=="result"` event, update Job with: summary(=result text), session_id, duration_ms, total_cost_usd, num_turns, is_error, stop_reason.
- If `total_cost_usd > MaxCostPerJobUSD` → mark job failed with `summary="cost-cap-exceeded"`, ExitCode set to a sentinel (e.g. 137-equivalent).
- Unknown event types → still persisted raw, worker does not crash.

**Non-functional**
- Scanner tolerates lines up to 4 MB.
- JobLog inserts batched in chunks of 50 (or flushed on scanner yield) to reduce SQLite write pressure.
- Sequence numbers monotonic per job.

## Architecture
```
internal/worker/
  runner.go            # now imports streamparser
  stream_parser.go     # new: parser + event dispatch
  log_writer.go        # new: batched JobLog inserts w/ seq counter

internal/model/
  stream_event.go      # new: typed structs for known event shapes
```

## Related code files
**Create:**
- `internal/worker/stream_parser.go`
- `internal/worker/log_writer.go`
- `internal/model/stream_event.go`

**Modify:**
- `internal/worker/runner.go` — swap `io.ReadAll` for parser + log_writer
- `internal/model/job.go` — confirm all summary fields present (added in phase 01)
- `internal/repository/job_log_repository.go` — add `CreateBatch([]JobLog)` helper

## Implementation steps
1. `internal/model/stream_event.go`: define:
   ```go
   type StreamEvent struct {
     Type    string          `json:"type"`
     Subtype string          `json:"subtype,omitempty"`
     Raw     json.RawMessage `json:"-"`
   }
   type ResultEvent struct {
     Type         string  `json:"type"`
     Subtype      string  `json:"subtype"`
     IsError      bool    `json:"is_error"`
     DurationMs   int     `json:"duration_ms"`
     TotalCostUSD float64 `json:"total_cost_usd"`
     SessionID    string  `json:"session_id"`
     NumTurns     int     `json:"num_turns"`
     Result       string  `json:"result"`
     StopReason   string  `json:"stop_reason"`
   }
   ```
2. `internal/worker/log_writer.go`:
   ```go
   type LogWriter struct {
     jobID  string
     seq    int
     buf    []model.JobLog
     repo   *repository.JobLogRepository
     // hub SSEHub — wired in phase 04
   }
   func (w *LogWriter) Append(stream, line string)  // push to buf, flush if >= 50
   func (w *LogWriter) Flush() error
   ```
   Always assigns monotonic `seq++` before append.
3. `internal/worker/stream_parser.go`:
   ```go
   type ParseResult struct {
     Final *ResultEvent  // nil if stream ended without a result event
   }
   func ParseStream(stdout io.Reader, lw *LogWriter) (*ParseResult, error)
   ```
   - `scanner := bufio.NewScanner(stdout); scanner.Buffer(make([]byte, 0, 1<<14), 4<<20)`
   - For each line:
     - `lw.Append("stdout", line)`
     - `json.Unmarshal(line, &StreamEvent{})` — ignore parse errors (still logged raw)
     - If `Type == "result"` → parse into `ResultEvent`, keep as final
   - Return `ParseResult{Final: ...}`, nil error unless scanner err.
4. stderr handled in a parallel goroutine:
   ```go
   go func() {
     s := bufio.NewScanner(stderr); s.Buffer(...)
     for s.Scan() { lw.Append("stderr", s.Text()) }
   }()
   ```
5. `runner.go` updated sequence:
   - Start cmd
   - stdoutPipe + stderrPipe
   - Launch stderr goroutine
   - `parseRes, parseErr := ParseStream(stdoutPipe, lw)`
   - `cmd.Wait()` (after pipes drained)
   - `lw.Flush()`
   - Determine final status:
     - ctx cancelled → `cancelled`
     - ctx deadline → `failed` summary=`timeout`
     - parseRes.Final != nil → apply cost guard:
       - `if total_cost_usd > cap`: status=`failed`, summary=`cost-cap-exceeded`
       - else: status from `is_error` (true → failed, false → done), summary=result text (truncated to ~2000 chars for display), populate all summary fields.
     - parseRes.Final == nil but exit 0 → failed, summary=`no-result-event`
     - exit != 0 and nil → failed, summary=last stderr tail
6. `job_log_repository.go`:
   ```go
   func (r *JobLogRepository) CreateBatch(logs []model.JobLog) error {
     if len(logs) == 0 { return nil }
     return r.db.CreateInBatches(logs, 50).Error
   }
   ```
7. Flush on final: if parser returns, call `lw.Flush()` before updating Job row so log ordering is consistent (all logs in DB before terminal status visible to UI).

## Todo list
- [x] model/stream_event.go typed structs
- [x] log_writer.go w/ batched inserts + seq counter
- [x] stream_parser.go scanner + 4MB buffer + event dispatch
- [x] stderr goroutine
- [x] runner.go rewritten to use parser
- [x] cost guard in runner final-status logic
- [x] job_log_repository.CreateBatch
- [x] Smoke: dummy job → inspect `job_logs` rows
- [x] Smoke: mocked high-cost result → verify cost-cap-exceeded
- [x] Smoke: force long line (>64KB) → no truncation/crash

## Success criteria
1. After a successful dummy job, `SELECT COUNT(*) FROM job_logs WHERE job_id=?` > 0 and rows have ascending `seq`.
2. Final `result` event populates `jobs.total_cost_usd`, `session_id`, `num_turns`, `duration_ms`, `summary`.
3. Forcing a cost above cap (e.g. set cap to 0.01) → job ends as `failed` with `summary='cost-cap-exceeded'`.
4. Wall-clock timeout (set to 5s, run a slow job) → `failed`, `summary='timeout'`.
5. Very long assistant message (test with a prompt like "print a 200 KB code block") → no scanner error, complete row in job_logs.
6. Unknown event type (inject a test line like `{"type":"future_event"}`) → still persisted, worker doesn't crash.

## Risks
| Risk | Mitigation |
|---|---|
| stream-json format drift across Claude versions | Use `StreamEvent{Type, Raw}` fallback; always persist raw line. Pin minimum CLI version in README. |
| Cost measured only at end (too late to save $) | Combine with wall-clock cap + document that cost cap is post-hoc. Future: pre-flight token estimate. |
| Batching delays log visibility in SSE (phase 04) | Also push each line to SSE hub synchronously before batching to DB — SSE fan-out immediate, DB write eventual. |
| SQLite write contention under bursty output | 50-row batches + `PRAGMA journal_mode=WAL` at DB open (add in phase 01 if not already) |
| Scanner buffer still too small | 4 MB handles all realistic Claude outputs; bump to 16 MB if issue observed |

## Security
- No new surface. All data confined to server's DB.
- Don't log JWT or env vars into job_logs (runner doesn't touch them).
- Truncate `summary` before writing to avoid unbounded column growth (done in code).

## Next steps
Phase 04 — SSE hub: expose `GET /api/jobs/:id/stream`, replay historical job_logs, then live fan-out from log_writer.
