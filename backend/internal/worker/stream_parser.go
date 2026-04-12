package worker

import (
	"bufio"
	"encoding/json"
	"io"
	"log"

	"github.com/mac/claudemote/backend/internal/model"
)

// ParseResult holds the outcome of scanning a stream-json stdout.
// Final is nil when the stream ended without a "result" event (e.g. crash).
type ParseResult struct {
	Final *model.ResultEvent // nil if no result event was seen
}

// ParseStream scans stdout line-by-line, persists every line via lw, and
// extracts the terminal "result" event when present.
//
// Scanner buffer is set to 4 MB to handle large assistant messages that would
// overflow the default 64 KB limit.
//
// Unknown event types are still logged raw — the worker never crashes on them.
func ParseStream(stdout io.Reader, lw *LogWriter) (*ParseResult, error) {
	scanner := bufio.NewScanner(stdout)
	// Initial token buffer 16 KB, max line size 4 MB.
	scanner.Buffer(make([]byte, 0, 1<<14), 4<<20)

	res := &ParseResult{}

	for scanner.Scan() {
		line := scanner.Text()
		lw.Append("stdout", line)

		// First-pass: detect event type without allocating a concrete struct.
		var ev model.StreamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Malformed JSON lines are still captured as raw log entries above.
			// Don't crash — Claude may emit non-JSON diagnostic lines too.
			log.Printf("stream_parser: non-JSON line (ignored): %v", err)
			continue
		}

		if ev.Type == "result" {
			var re model.ResultEvent
			if err := json.Unmarshal([]byte(line), &re); err != nil {
				log.Printf("stream_parser: failed to parse result event: %v", err)
				continue
			}
			res.Final = &re
		}
	}

	if err := scanner.Err(); err != nil {
		return res, err
	}
	return res, nil
}
