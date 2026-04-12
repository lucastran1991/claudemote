package model

import "time"

// Job represents a single Claude Code invocation queued through the API.
// status lifecycle: pending → running → done | failed | cancelled
type Job struct {
	ID           string     `gorm:"primaryKey"  json:"id"`         // uuid
	Command      string     `gorm:"not null"    json:"command"`    // free-text prompt
	Model        string     `gorm:"not null"    json:"model"`      // sonnet|opus|haiku
	Status       string     `gorm:"not null"    json:"status"`     // pending|running|done|failed|cancelled
	ExitCode     *int       `                   json:"exit_code"`  // nil until finished
	Summary      string     `                   json:"summary"`    // from final result event
	SessionID    string     `                   json:"session_id"` // Claude Code session id
	DurationMs   int        `                   json:"duration_ms"`
	TotalCostUSD float64    `                   json:"total_cost_usd"`
	NumTurns     int        `                   json:"num_turns"`
	IsError      bool       `                   json:"is_error"`
	StopReason   string     `                   json:"stop_reason"`
	CreatedAt    time.Time  `                   json:"created_at"`
	StartedAt    *time.Time `                   json:"started_at"`  // nil until worker picks up
	FinishedAt   *time.Time `                   json:"finished_at"` // nil until complete
}

// JobLog stores one line of stream-json output from a running Job.
// seq is monotonically increasing per job — used for ordered SSE replay.
type JobLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JobID     string    `gorm:"not null;index:idx_job_logs_job_id_seq,priority:1" json:"job_id"`
	Seq       int       `gorm:"not null;index:idx_job_logs_job_id_seq,priority:2" json:"seq"`
	Stream    string    `gorm:"not null"   json:"stream"` // stdout | stderr
	Line      string    `gorm:"not null"   json:"line"`   // raw stream-json line
	CreatedAt time.Time `                  json:"created_at"`
}
