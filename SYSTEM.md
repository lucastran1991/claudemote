# Remote Claude Code Controller

## Overview
A web app to remotely trigger and monitor Claude Code tasks on an AWS EC2 instance from any browser or iOS device.

## Architecture

```
iOS/Browser
    |
Next.js (frontend)
    |
    ├── POST /run     → submit a task
    ├── GET  /jobs    → list all jobs
    └── GET  /jobs/:id → job detail
    |
Go API server (EC2)
    |
    ├── Job queue (Go channel, in-memory)
    ├── Executes: claude -p "<command>" --output-format json
    ├── Reads/writes codebase files on EC2
    └── SQLite (job history)
```

## Data Flow
1. User submits a command (free text or preset task)
2. Go API creates a job, queues it, returns `job_id` immediately
3. Worker goroutine runs `claude -p` as subprocess
4. Result (status + short summary) saved to SQLite
5. Frontend polls `GET /jobs` every few seconds to show updates

## Job Schema
```json
{
  "id": "uuid",
  "command": "fix bug in auth.go",
  "status": "pending | running | done | failed",
  "summary": "Fixed null pointer in login handler",
  "created_at": "2026-04-12T15:00:00Z",
  "finished_at": "2026-04-12T15:01:30Z"
}
```

## Stack
- **Backend**: Go — HTTP server, job queue, subprocess runner
- **Frontend**: Next.js — command input, job history table
- **Database**: SQLite — lightweight, single-file, no setup
- **AI**: Claude Code CLI (`claude -p`) — non-interactive mode

## Key Constraints
- No auth (private use only)
- No log streaming — status + summary only
- One job runs at a time (single worker goroutine)
- Claude Code must already be installed and authenticated on EC2
