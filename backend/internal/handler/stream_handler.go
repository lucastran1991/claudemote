package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/internal/repository"
	"github.com/mac/claudemote/backend/internal/sse"
	"github.com/mac/claudemote/backend/pkg/response"
)

// terminalStatuses is the set of job statuses that mean no more lines will arrive.
var terminalStatuses = map[string]bool{
	"done":      true,
	"failed":    true,
	"cancelled": true,
}

// StreamHandler serves GET /api/jobs/:id/stream as a Server-Sent Events endpoint.
// It replays historical job_logs first (honouring Last-Event-ID for resume), then
// subscribes to the in-memory Hub and forwards live lines until the job is terminal
// or the client disconnects.
type StreamHandler struct {
	jobRepo *repository.JobRepository
	logRepo *repository.JobLogRepository
	hub     *sse.Hub
}

// NewStreamHandler wires the handler to its dependencies.
func NewStreamHandler(
	jobRepo *repository.JobRepository,
	logRepo *repository.JobLogRepository,
	hub *sse.Hub,
) *StreamHandler {
	return &StreamHandler{jobRepo: jobRepo, logRepo: logRepo, hub: hub}
}

// Stream handles GET /api/jobs/:id/stream.
func (h *StreamHandler) Stream(c *gin.Context) {
	jobID := c.Param("id")

	job, err := h.jobRepo.FindByID(jobID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "job not found")
		return
	}

	lastID := parseLastEventID(c)

	// Set SSE response headers before writing any body bytes.
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Disable Nginx/Caddy proxy buffering so bytes reach the client immediately.
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// Extremely unlikely — Gin's ResponseWriter always wraps net/http.
		response.Error(c, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// --- Subscribe FIRST to avoid a replay gap ---
	// Subscribing before the history query ensures that any events published
	// while we are reading from the DB go into the subscriber channel buffer
	// (capacity 256) and are not lost. We deduplicate against history by seq
	// number below before forwarding live events.
	//
	// For terminal jobs we still subscribe so the code path is uniform; the
	// channel is unsubscribed immediately after we return below.
	ch, unsub := h.hub.Subscribe(jobID)
	defer unsub()

	// --- Replay historical lines ---
	logs, err := h.logRepo.ListAfterSeq(jobID, lastID)
	if err != nil {
		// Write error as SSE comment so the client can surface it, then close.
		fmt.Fprintf(c.Writer, ": error loading history: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Track the highest seq emitted from history so we can skip duplicates
	// that were buffered in ch during the DB query.
	maxHistorySeq := -1
	for _, lg := range logs {
		writeSSEEvent(c.Writer, lg.Seq, lg.Line)
		flusher.Flush()
		if lg.Seq > maxHistorySeq {
			maxHistorySeq = lg.Seq
		}
	}

	// Job already in terminal state — history sent, we are done.
	if terminalStatuses[job.Status] {
		return
	}

	// --- Drain buffered live events, skipping anything already in history ---
	// Flush the channel of events that arrived during the DB query, deduplicating
	// against history by seq so the client never sees the same line twice.
	ctx := c.Request.Context()
	eventCount := 0

drainLoop:
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Seq > maxHistorySeq {
				writeSSEEvent(c.Writer, ev.Seq, ev.Line)
				flusher.Flush()
				eventCount++
			}
		default:
			// Channel is empty — transition to blocking select below.
			break drainLoop
		}
	}

	// --- Stream live lines until terminal or client disconnect ---
	for {
		select {
		case <-ctx.Done():
			// Client disconnected.
			return

		case ev, ok := <-ch:
			if !ok {
				// Hub closed the channel (unsub was called, should not normally happen here).
				return
			}
			writeSSEEvent(c.Writer, ev.Seq, ev.Line)
			flusher.Flush()
			eventCount++

			// Periodically re-check job status so we close cleanly when the worker
			// finishes even if no more events arrive (e.g. final flush before done).
			if eventCount%20 == 0 {
				cur, dbErr := h.jobRepo.FindByID(jobID)
				if dbErr == nil && terminalStatuses[cur.Status] {
					return
				}
			}
		}
	}
}

// parseLastEventID reads the resume cursor from the Last-Event-ID header first,
// then falls back to the ?lastEventId= query parameter. Returns 0 (send all) if
// neither is present or parseable.
func parseLastEventID(c *gin.Context) int {
	raw := c.GetHeader("Last-Event-ID")
	if raw == "" {
		raw = c.Query("lastEventId")
	}
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}

// writeSSEEvent writes one SSE event block to w.
// Newlines inside data are escaped to "\\n" (literal backslash-n) so the SSE
// framing is never broken. Stream-json lines are always single-line in practice,
// but this guard keeps the protocol correct for any edge case.
func writeSSEEvent(w io.Writer, seq int, data string) {
	safe := strings.ReplaceAll(data, "\n", `\n`)
	fmt.Fprintf(w, "id: %d\ndata: %s\n\n", seq, safe)
}
