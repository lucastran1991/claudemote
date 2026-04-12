package worker

import (
	"log"
	"sync"

	"github.com/mac/claudemote/backend/internal/model"
	"github.com/mac/claudemote/backend/internal/repository"
)

const logFlushThreshold = 50

// LogWriter accumulates job log lines in memory and batch-inserts them into
// job_logs. It also fans out each line to the SSE hub (if wired) immediately,
// so live streaming is not delayed by the DB batch window.
//
// mu guards seq, buf, and all calls to repo/hub so that the stdout parser
// goroutine and the stderr reader goroutine can safely call Append concurrently
// (see runner.go — both goroutines run simultaneously).
type LogWriter struct {
	mu    sync.Mutex
	jobID string
	seq   int
	buf   []model.JobLog
	repo  *repository.JobLogRepository
	hub   SSEPublisher // nil until phase 04 wires the real hub
}

// newLogWriter constructs a LogWriter for the given job.
func newLogWriter(jobID string, repo *repository.JobLogRepository, hub SSEPublisher) *LogWriter {
	return &LogWriter{
		jobID: jobID,
		repo:  repo,
		hub:   hub,
		buf:   make([]model.JobLog, 0, logFlushThreshold),
	}
}

// Append records one log line. It assigns the next monotonic sequence number,
// publishes immediately to the SSE hub (nil-safe), and buffers for batch DB
// insert. Auto-flushes when the buffer reaches logFlushThreshold.
// Safe to call concurrently from multiple goroutines.
func (w *LogWriter) Append(stream, line string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := model.JobLog{
		JobID:  w.jobID,
		Seq:    w.seq,
		Stream: stream,
		Line:   line,
	}
	w.seq++
	w.buf = append(w.buf, entry)

	// Fan-out to SSE hub immediately (non-blocking write path, hub is nil-safe).
	if w.hub != nil {
		w.hub.Publish(w.jobID, entry.Seq, line)
	}

	// Flush the batch once it reaches the threshold to reduce write pressure.
	// flushLocked is called here while mu is already held.
	if len(w.buf) >= logFlushThreshold {
		if err := w.flushLocked(); err != nil {
			log.Printf("log_writer: flush error for job %s: %v", w.jobID, err)
		}
	}
}

// Flush batch-inserts all buffered log lines into the database and clears the
// buffer. Safe to call when the buffer is empty (no-op). Always call Flush
// after the subprocess exits to drain any remaining lines before updating the
// job's terminal state.
func (w *LogWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// flushLocked performs the actual flush. Caller must hold w.mu.
func (w *LogWriter) flushLocked() error {
	if len(w.buf) == 0 {
		return nil
	}
	if err := w.repo.CreateBatch(w.buf); err != nil {
		return err
	}
	w.buf = w.buf[:0]
	return nil
}
