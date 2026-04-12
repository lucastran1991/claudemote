package service

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mac/claudemote/backend/internal/model"
	"github.com/mac/claudemote/backend/internal/repository"
)

var (
	// ErrJobNotFound is returned when a job ID does not exist.
	ErrJobNotFound = errors.New("job not found")
	// ErrJobNotCancellable is returned when a job is already finished.
	ErrJobNotCancellable = errors.New("job is not in a cancellable state")
	// ErrQueueFull is returned when the worker pool channel is at capacity.
	ErrQueueFull = errors.New("worker queue is full, try again later")
)

// Pooler is the subset of worker.Pool used by JobService.
// Defined here to avoid an import cycle (service → worker → repository → service).
type Pooler interface {
	Enqueue(jobID string) error
	Cancel(jobID string) bool
}

// JobService manages job lifecycle.
type JobService struct {
	jobRepo      *repository.JobRepository
	jobLogRepo   *repository.JobLogRepository
	defaultModel string
	pool         Pooler // nil until Phase 02 wires the worker pool
}

// NewJobService creates a JobService with the given repositories and defaults.
func NewJobService(
	jobRepo *repository.JobRepository,
	jobLogRepo *repository.JobLogRepository,
	defaultModel string,
) *JobService {
	return &JobService{
		jobRepo:      jobRepo,
		jobLogRepo:   jobLogRepo,
		defaultModel: defaultModel,
	}
}

// SetPool injects the worker pool after construction (avoids circular init).
// Called by main.go once the pool is built.
func (s *JobService) SetPool(p Pooler) {
	s.pool = p
}

// Enqueue creates a new job in pending state, persists it, then pushes the ID
// onto the worker pool channel so a goroutine picks it up immediately.
func (s *JobService) Enqueue(command, modelName string) (*model.Job, error) {
	if modelName == "" {
		modelName = s.defaultModel
	}

	job := &model.Job{
		ID:      uuid.New().String(),
		Command: command,
		Model:   modelName,
		Status:  "pending",
	}

	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	// Phase 02: push to worker pool. pool is nil only in unit tests / phase 01.
	if s.pool != nil {
		if err := s.pool.Enqueue(job.ID); err != nil {
			// Queue is full — delete the row we just inserted so it does not
			// linger as an orphan "pending" job that Recover() would replay on
			// the next boot, causing duplicate execution.
			_ = s.jobRepo.Delete(job.ID)
			return nil, ErrQueueFull
		}
	}

	return job, nil
}

// List returns all jobs ordered by created_at DESC.
func (s *JobService) List() ([]model.Job, error) {
	return s.jobRepo.List()
}

// Get retrieves a single job by ID.
func (s *JobService) Get(id string) (*model.Job, error) {
	job, err := s.jobRepo.FindByID(id)
	if err != nil {
		return nil, ErrJobNotFound
	}
	return job, nil
}

// Cancel signals a pending or running job to stop.
// For pending jobs it writes cancelled directly to DB.
// For running jobs it fires the subprocess context cancel via the pool;
// the worker goroutine will update the DB when the process exits.
func (s *JobService) Cancel(id string) error {
	job, err := s.jobRepo.FindByID(id)
	if err != nil {
		return ErrJobNotFound
	}

	if job.Status != "pending" && job.Status != "running" {
		return ErrJobNotCancellable
	}

	// Running jobs: let the worker update the DB after the process exits.
	if job.Status == "running" && s.pool != nil {
		s.pool.Cancel(id)
		return nil
	}

	// Pending jobs (not yet picked up by a worker): cancel in DB immediately.
	now := time.Now()
	job.Status = "cancelled"
	job.FinishedAt = &now
	return s.jobRepo.Update(job)
}
