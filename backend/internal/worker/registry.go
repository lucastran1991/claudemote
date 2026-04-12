package worker

import (
	"context"
	"sync"
)

// Registry is a concurrency-safe map of jobID → context.CancelFunc.
// It tracks in-flight jobs so cancel requests can interrupt them.
type Registry struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// newRegistry constructs an empty Registry.
func newRegistry() *Registry {
	return &Registry{
		cancels: make(map[string]context.CancelFunc),
	}
}

// Set registers a cancel function for the given job ID.
// Replaces any existing entry (should not happen under normal operation).
func (r *Registry) Set(id string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[id] = cancel
}

// Remove deletes the cancel entry for id. Called when a job finishes.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, id)
}

// Cancel calls the cancel function for id and removes it from the map.
// Returns true if the job was found and cancelled, false if not registered.
func (r *Registry) Cancel(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn, ok := r.cancels[id]
	if !ok {
		return false
	}
	fn()
	delete(r.cancels, id)
	return true
}
