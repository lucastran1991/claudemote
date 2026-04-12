package sse

import (
	"log"
	"sync"
)

// Event is one SSE message published from a worker's LogWriter.
type Event struct {
	Seq  int
	Line string
}

// Hub is an in-memory pub/sub broker keyed by job ID.
// It satisfies the worker.SSEPublisher interface.
// Publish is non-blocking: if a subscriber's channel is full the event is dropped
// so the worker scanner is never stalled by a slow client.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[chan Event]struct{}
}

// NewHub initialises an empty Hub ready for use.
func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan Event]struct{}),
	}
}

// Subscribe registers a new subscriber for jobID.
// Returns a receive-only channel (buffered 256) and a cleanup func.
// The caller MUST call the cleanup func when done (deferred in the handler).
func (h *Hub) Subscribe(jobID string) (<-chan Event, func()) {
	ch := make(chan Event, 256)

	h.mu.Lock()
	if h.subs[jobID] == nil {
		h.subs[jobID] = make(map[chan Event]struct{})
	}
	h.subs[jobID][ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.subs[jobID], ch)
		if len(h.subs[jobID]) == 0 {
			delete(h.subs, jobID)
		}
		h.mu.Unlock()
		close(ch)
	}
	return ch, unsub
}

// Publish fans out an event to all current subscribers for jobID.
// Uses a non-blocking send: full channels are skipped and a warning is logged.
// Acquires RLock so multiple goroutines may publish concurrently.
func (h *Hub) Publish(jobID string, seq int, line string) {
	ev := Event{Seq: seq, Line: line}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subs[jobID] {
		select {
		case ch <- ev:
		default:
			// Slow consumer — drop rather than block the worker scanner.
			log.Printf("sse: dropped event seq=%d for job %s (subscriber buffer full)", seq, jobID)
		}
	}
}
