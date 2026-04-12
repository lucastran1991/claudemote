// Hand-written types matching backend internal/model/job.go
// Authoritative source: /Users/mac/studio/claudemote/backend/internal/model/job.go

// JobStatus mirrors the backend lifecycle: pending → running → done | failed | cancelled
export type JobStatus = "pending" | "running" | "done" | "failed" | "cancelled"

// JobModel mirrors the model identifiers accepted by the backend
export type JobModel = "claude-sonnet-4-6" | "claude-opus-4-6" | "claude-haiku-4-5-20251001"

// Job mirrors model.Job (json tags)
export interface Job {
  id: string            // uuid
  command: string       // free-text prompt sent to Claude Code
  model: JobModel       // model identifier
  status: JobStatus
  exit_code: number | null     // null until finished
  summary: string              // from final result event
  session_id: string           // Claude Code session id
  duration_ms: number
  total_cost_usd: number
  num_turns: number
  is_error: boolean
  stop_reason: string
  created_at: string           // ISO 8601
  started_at: string | null    // null until worker picks up
  finished_at: string | null   // null until complete
}

// JobLog mirrors model.JobLog (json tags)
export interface JobLog {
  id: number
  job_id: string
  seq: number          // monotonically increasing per job — used for SSE replay ordering
  stream: "stdout" | "stderr"
  line: string         // raw stream-json line from Claude Code
  created_at: string   // ISO 8601
}

// API response shapes used by job endpoints (phase 06)
export interface JobListResponse {
  jobs: Job[]
  total: number
}

export interface CreateJobRequest {
  command: string
  model: string
}
