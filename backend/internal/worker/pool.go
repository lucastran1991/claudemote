package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mac/claudemote/backend/internal/config"
	"github.com/mac/claudemote/backend/internal/repository"
	"gorm.io/gorm"
)

const jobChannelBuffer = 100

// SSEPublisher is implemented by the SSE hub in phase 04.
// Pool accepts a nil value — runner checks before calling.
type SSEPublisher interface {
	Publish(jobID string, seq int, line string)
}

// Pool manages a fixed set of worker goroutines that consume job IDs from a
// buffered channel and execute them as claude -p subprocesses.
type Pool struct {
	jobs    chan string
	wg      sync.WaitGroup
	cfg     *config.Config
	db      *gorm.DB
	jobRepo *repository.JobRepository
	logRepo *repository.JobLogRepository
	reg     *Registry
	hub     SSEPublisher // nil until phase 04
}

// New constructs a Pool. hub may be nil (phase 04 wires the real SSE hub).
func New(
	cfg *config.Config,
	db *gorm.DB,
	jobRepo *repository.JobRepository,
	logRepo *repository.JobLogRepository,
	hub SSEPublisher,
) *Pool {
	return &Pool{
		jobs:    make(chan string, jobChannelBuffer),
		cfg:     cfg,
		db:      db,
		jobRepo: jobRepo,
		logRepo: logRepo,
		reg:     newRegistry(),
		hub:     hub,
	}
}

// Start spawns cfg.WorkerCount goroutines. They exit when ctx is cancelled.
// Call Recover() before Start() to re-queue persisted pending jobs.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.cfg.WorkerCount; i++ {
		i := i
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			log.Printf("worker %d started", i)
			for {
				select {
				case <-ctx.Done():
					log.Printf("worker %d shutting down", i)
					return
				case jobID, ok := <-p.jobs:
					if !ok {
						return
					}
					runJob(ctx, jobID, p.cfg, p.jobRepo, p.logRepo, p.reg, p.hub)
				}
			}
		}()
	}
}

// Enqueue pushes a job ID onto the work channel.
// Returns an error (caller should respond 503) if the channel buffer is full.
func (p *Pool) Enqueue(jobID string) error {
	select {
	case p.jobs <- jobID:
		return nil
	default:
		return fmt.Errorf("worker pool queue full (capacity %d)", jobChannelBuffer)
	}
}

// Cancel sends a cancel signal to a running job via the Registry.
// Returns true if the job was found and signalled.
func (p *Pool) Cancel(jobID string) bool {
	return p.reg.Cancel(jobID)
}

// Recover must be called once before Start.
// It marks any jobs left in "running" state as failed (server crashed mid-job),
// then re-enqueues all "pending" jobs in created_at order.
func (p *Pool) Recover() error {
	jobs, err := p.jobRepo.ListRecoverable()
	if err != nil {
		return fmt.Errorf("recover: list recoverable jobs: %w", err)
	}

	now := time.Now()
	for _, j := range jobs {
		j := j
		if j.Status == "running" {
			// Mark crashed-running jobs as failed so they surface in the UI.
			summary := "recovered-after-crash"
			j.Status = "failed"
			j.Summary = summary
			j.FinishedAt = &now
			if err := p.jobRepo.Update(&j); err != nil {
				log.Printf("recover: failed to update job %s: %v", j.ID, err)
			}
			log.Printf("recover: marked crashed job %s as failed", j.ID)
		} else if j.Status == "pending" {
			// Re-enqueue pending jobs; if the channel is full log and skip.
			if err := p.Enqueue(j.ID); err != nil {
				log.Printf("recover: could not re-enqueue job %s: %v", j.ID, err)
			} else {
				log.Printf("recover: re-enqueued pending job %s", j.ID)
			}
		}
	}
	return nil
}
