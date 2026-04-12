---
type: brainstorm
project: claudemote
date: 2026-04-12
status: approved
---

# Brainstorm: Remote Claude Code Controller (claudemote)

## Problem

Need a web app to remotely queue + monitor Claude Code jobs on an EC2 box, from browser or iOS. Draft `SYSTEM.md` exists but several choices would bite under real use.

## Requirements (confirmed with user)

- **Auth**: reuse playground NextAuth + JWT. Admin single-user. JWT bearer on API.
- **Concurrency**: worker pool `N` goroutines, env-configurable (default 2).
- **Logs**: full output stored in DB + SSE live stream on detail page.
- **Repo scope**: single pinned working directory (v1). Multi-repo = v2.
- **Target**: EC2 (confirmed `claude -p` headless works with `bypassPermissions`).
- **Reference stack**: mirror `/Users/mac/studio/playground` (Gin+GORM+SQLite+JWT / Next.js App Router+React Query+NextAuth+shadcn+Tailwind / pm2+Caddy+Makefile).

## Rejected options

| Rejected | Why |
|---|---|
| No auth, IP allowlist only | Public endpoint executing arbitrary shell = unacceptable. |
| Single worker goroutine | 10-30min jobs queue painfully. |
| In-memory Go channel queue only | Jobs vanish on restart. |
| Summary-only logs | Loses all Claude tool-call visibility; debugging impossible. |
| `--output-format json` (one final blob) | Blocks SSE streaming; switched to `stream-json`. |
| Opus default | $0.23 empirical per trivial call. Unsustainable. Sonnet default. |

## Empirical cost data

User tested on EC2: `claude -p "list files" --output-format json --permission-mode bypassPermissions`
- Duration: 11.4s
- Cost: **$0.229 USD** (Opus 4.6, 1M ctx)
- Cache creation: 32,505 tokens (project CLAUDE.md tax paid every invocation)
- `permission_denials: []`, `terminal_reason: completed` → headless verified

## Final architecture

```
iOS/Browser ──HTTPS──► Caddy :443 ─► Next.js :3000 (UI, NextAuth)
                                ─► Go API :8080 (JWT-protected)
                                      │
            POST   /api/jobs                enqueue
            GET    /api/jobs                list (paginated)
            GET    /api/jobs/:id            detail + stored logs
            POST   /api/jobs/:id/cancel     SIGTERM subprocess
            GET    /api/jobs/:id/stream     SSE live tail
                                      │
                                 SQLite (jobs + job_logs)
                                      │
                                 Worker pool (N goroutines)
                                      │
           exec.CommandContext("claude", "-p", cmd,
              "--output-format", "stream-json",
              "--permission-mode", "bypassPermissions",
              "--model", model)
           cwd = WORK_DIR
```

### Schema

```go
type Job struct {
  ID           string     // uuid
  Command      string     // free-text prompt
  Model        string     // sonnet|opus|haiku (default sonnet)
  Status       string     // pending|running|done|failed|cancelled
  ExitCode     *int
  Summary      string     // from final result event
  SessionID    string     // Claude Code session id (v2 resume)
  DurationMs   int
  TotalCostUSD float64
  NumTurns     int
  IsError      bool
  StopReason   string
  CreatedAt    time.Time
  StartedAt    *time.Time
  FinishedAt   *time.Time
}

type JobLog struct {
  ID        uint
  JobID     string     // FK
  Seq       int
  Stream    string     // stdout|stderr
  Line      string     // one stream-json line
  CreatedAt time.Time
}
```

### Worker pool

- On boot: `N` goroutines read from `chan string` of job IDs.
- `enqueue()`: INSERT pending job → push id to channel.
- Crash recovery: on boot, mark any `running` as `failed`, re-queue `pending`.
- Job execution: `exec.CommandContext` w/ per-job ctx → scan stdout line-by-line → append `JobLog` + broadcast to SSE hub → on final `result` event, parse summary fields → mark done.
- Cancel: `cancel()` on ctx → SIGTERM.
- Cost guard: parse `total_cost_usd` from incremental events; if exceeds `MAX_COST_PER_JOB_USD`, cancel.
- Wall-clock guard: per-job timeout (`JOB_TIMEOUT_MIN`, default 30).

### SSE hub

- `map[jobID][]chan string`, mutex-protected.
- Subscriber first replays historical `JobLog` rows from DB, then relays live lines.
- Reconnect via `Last-Event-ID` header (SSE standard).
- ~80 LOC, no external deps.

### Frontend (mirrors playground)

- `(auth)/login` — NextAuth Credentials, copy from playground.
- `(dashboard)/jobs` — React Query list, poll 3s when any running else 30s.
- `(dashboard)/jobs/[id]` — EventSource live stream → `<pre>` terminal. Cancel button. Cost + duration badges.
- `(dashboard)/new` — textarea + model dropdown (sonnet default) + submit. Mobile-first big button.

### Config (env)

```
WORKER_COUNT=2
WORK_DIR=/opt/atomiton/playwright-demo
CLAUDE_BIN=/usr/local/bin/claude
CLAUDE_DEFAULT_MODEL=claude-sonnet-4-6
CLAUDE_PERMISSION_MODE=bypassPermissions
JOB_TIMEOUT_MIN=30
MAX_COST_PER_JOB_USD=1.00
JOB_LOG_RETENTION_DAYS=14
JWT_SECRET=...
ADMIN_USERNAME=...
ADMIN_PASSWORD_HASH=...
```

### Ops

- `start.sh` + pm2 + `ecosystem.config.cjs` lifted from playground.
- `make dev`, `make build`, `make migrate-up` identical conventions.
- Caddy reverse proxy with `flush_interval -1` for SSE.
- Deploy: `git pull && make build && pm2 reload all`.

## Risks + mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| Runaway API cost | **High** | Sonnet default + `MAX_COST_PER_JOB_USD` cap + wall-clock timeout + cancel button |
| 32k token CLAUDE.md tax per job | Medium | Out of scope here, flagged as separate follow-up for playwright-demo repo |
| SQLite writer contention | Low | GORM serializes. Only an issue above N≈10 workers. |
| SSE through Caddy | Low | `flush_interval -1` documented. |
| JobLog table growth | Medium | Daily cron deletes `> JOB_LOG_RETENTION_DAYS`. |
| Subprocess zombie on crash | Low | `ProcessGroup` + kill on ctx cancel. |

## Out of v1 (deferred)

- Preset / saved commands
- Multi-repo selection
- Push notifications
- Session resume (Claude Code `--resume <session_id>`)
- Structured rendering of tool-calls (edits, bash) — v1 dumps raw stream-json
- Git auto-commit of Claude's changes

## Success criteria

1. Login via NextAuth, JWT issued.
2. Submit free-text command → job queued → subprocess starts.
3. Live SSE stream shows `claude -p --output-format stream-json` output in real time.
4. Job completes → cost + duration + summary visible.
5. Cancel button terminates running job within 5s.
6. Server restart: in-flight jobs marked failed, pending jobs resume.
7. Cost cap: job exceeding `MAX_COST_PER_JOB_USD` is auto-cancelled.

## Build order

1. Scaffold Go backend cloned from playground layout. Swap models/handlers/services for `Job`.
2. Worker pool + subprocess runner: `claude -p` end-to-end against a dummy prompt.
3. stream-json parser + JobLog persistence + cost/timeout guards.
4. SSE hub + `/api/jobs/:id/stream`.
5. Frontend scaffold from playground, strip unused user management.
6. Jobs list + detail pages + new-job form.
7. Deploy pipeline: pm2 + Caddy on EC2.

## Open follow-ups (not blocking)

- Trim `/opt/atomiton/playwright-demo/CLAUDE.md` to reduce per-job cache-creation tax.
- Verify exact Caddy config for SSE passthrough.
- Decide v2 scope: presets, multi-repo, session resume, push notif.
