package repository

import (
	"github.com/mac/claudemote/backend/internal/model"
	"gorm.io/gorm"
)

// JobLogRepository handles database operations for the JobLog model.
type JobLogRepository struct {
	db *gorm.DB
}

// NewJobLogRepository creates a new JobLogRepository backed by db.
func NewJobLogRepository(db *gorm.DB) *JobLogRepository {
	return &JobLogRepository{db: db}
}

// Create inserts a new job log line.
func (r *JobLogRepository) Create(log *model.JobLog) error {
	return r.db.Create(log).Error
}

// FindByJobID retrieves all log lines for a job ordered by seq ascending.
// Used by SSE handler to replay historical output before streaming live lines.
func (r *JobLogRepository) FindByJobID(jobID string) ([]model.JobLog, error) {
	var logs []model.JobLog
	if err := r.db.Where("job_id = ?", jobID).
		Order("seq ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// CreateBatch inserts a slice of JobLog rows in chunks of 50.
// No-op if logs is empty.
func (r *JobLogRepository) CreateBatch(logs []model.JobLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.CreateInBatches(logs, 50).Error
}

// ListAfterSeq returns all log lines for a job with seq > afterSeq, ordered ascending.
// Used by the SSE stream handler for Last-Event-ID resume: pass the last seen seq
// and only lines after that point are replayed, avoiding redundant resend.
func (r *JobLogRepository) ListAfterSeq(jobID string, afterSeq int) ([]model.JobLog, error) {
	var logs []model.JobLog
	if err := r.db.Where("job_id = ? AND seq > ?", jobID, afterSeq).
		Order("seq ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// DeleteOlderThan removes log rows whose job finished more than retentionDays ago.
// Intended for a nightly cleanup cron (Phase 07).
func (r *JobLogRepository) DeleteOlderThan(retentionDays int) error {
	return r.db.Exec(
		`DELETE FROM job_logs WHERE job_id IN (
			SELECT id FROM jobs
			WHERE finished_at IS NOT NULL
			  AND finished_at < datetime('now', ? || ' days')
		)`,
		-retentionDays,
	).Error
}
