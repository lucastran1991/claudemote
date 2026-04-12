package repository

import (
	"github.com/mac/claudemote/backend/internal/model"
	"gorm.io/gorm"
)

// JobRepository handles database operations for the Job model.
type JobRepository struct {
	db *gorm.DB
}

// NewJobRepository creates a new JobRepository backed by db.
func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new job row.
func (r *JobRepository) Create(job *model.Job) error {
	return r.db.Create(job).Error
}

// FindByID retrieves a job by its uuid primary key.
func (r *JobRepository) FindByID(id string) (*model.Job, error) {
	var job model.Job
	if err := r.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// List returns all jobs ordered by created_at descending.
// Phase 02 will add pagination — kept simple for scaffold.
func (r *JobRepository) List() ([]model.Job, error) {
	var jobs []model.Job
	if err := r.db.Order("created_at DESC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// Update persists changes to an existing job row.
func (r *JobRepository) Update(job *model.Job) error {
	return r.db.Save(job).Error
}

// Delete removes a job by id.
func (r *JobRepository) Delete(id string) error {
	return r.db.Delete(&model.Job{}, "id = ?", id).Error
}

// ListRecoverable returns jobs in pending or running state.
// Called on boot so Phase 02 worker pool can re-queue pending and
// mark running jobs as failed (crash recovery).
func (r *JobRepository) ListRecoverable() ([]model.Job, error) {
	var jobs []model.Job
	if err := r.db.Where("status IN ?", []string{"pending", "running"}).
		Order("created_at ASC").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}
