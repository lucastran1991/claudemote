# Phase 02 — Worker Pool + `claude -p` Subprocess Runner

## Context
- Depends on: phase-01-backend-scaffold
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md

## Overview
**Priority:** P0
**Status:** pending
**Effort:** M

Worker pool of N goroutines consuming queued jobs. Each job spawns a `claude -p` subprocess and captures its output. No log parsing yet (Phase 03 adds that) — this phase just proves end-to-end subprocess lifecycle: enqueue → exec → exit → mark done.

## Key insights
- `exec.CommandContext` + process group (`Setpgid: true`) is the cleanest cancel story on Linux.
- Job recovery on boot is critical — a crashed server must not lose pending jobs or leave zombies in `running` state.
- The cancel map is a separate in-memory structure; not persisted. Reboot wipes it and crash-recovery takes over.
- On EC2 the `claude` binary must be authenticated for the same user running the Go server. Document this in README.

## Requirements
**Functional**
- `WORKER_COUNT=N` goroutines start at server boot; each reads from a shared `chan string` of job IDs.
- `JobService.Enqueue(command, model)` inserts pending row, returns job, pushes id to channel.
- Worker transitions job: `pending → running → (done|failed|cancelled)`.
- `POST /api/jobs/:id/cancel` triggers ctx cancel → SIGTERM propagates to subprocess group within 5s.
- Boot recovery: `UPDATE jobs SET status='failed', finished_at=NOW() WHERE status='running'` then re-enqueue `pending` rows in created_at order.

**Non-functional**
- No leaked goroutines on server shutdown (context cancel propagates).
- No zombie subprocesses on cancel (process group kill).

## Architecture
```
internal/worker/
  pool.go           # Pool struct, Start(ctx), Enqueue(jobID), Cancel(jobID)
  runner.go         # runJob(ctx, job, cfg, repos) — exec + capture
  registry.go       # jobID → CancelFunc map, mutex-protected
```

Wiring:
- `JobService` holds `*worker.Pool`, calls `Enqueue` after DB insert.
- `main.go` creates pool, calls `pool.Recover()` then `pool.Start(ctx)` before `router.Setup`.

## Related code files
**Create:**
- `internal/worker/pool.go`
- `internal/worker/runner.go`
- `internal/worker/registry.go`

**Modify:**
- `internal/service/job_service.go` — inject pool, call `Enqueue` post-insert
- `internal/handler/job_handler.go` — implement `POST /jobs` + cancel
- `cmd/server/main.go` — construct pool, call `Recover()` + `Start(ctx)`

## Implementation steps
1. `internal/worker/registry.go`:
   ```go
   type Registry struct { mu sync.Mutex; cancels map[string]context.CancelFunc }
   func (r *Registry) Set(id, cancel); Get(id); Remove(id); Cancel(id) bool
   ```
2. `internal/worker/pool.go`:
   ```go
   type Pool struct {
     jobs chan string
     wg   sync.WaitGroup
     db   *gorm.DB
     cfg  *config.Config
     reg  *Registry
     repo *repository.JobRepository
     logs *repository.JobLogRepository
     // hub  SSEHub  (wired in phase 04 via interface)
   }
   func New(cfg, db, ...) *Pool
   func (p *Pool) Start(ctx context.Context)  // spawns N workers
   func (p *Pool) Enqueue(jobID string)
   func (p *Pool) Cancel(jobID string) bool
   func (p *Pool) Recover() error  // called once before Start
   ```
3. `Recover()`: in a tx, set running→failed with `summary="recovered-after-crash"`, then select pending jobs and enqueue.
4. Worker loop:
   - `for jobID := range p.jobs { p.runOne(ctx, jobID) }`
   - On parent ctx cancel, channel drains and workers exit.
5. `runner.go runJob(parentCtx, jobID, ...)`:
   - Load job from DB; bail if not pending.
   - Mark running, set StartedAt.
   - `jobCtx, cancel := context.WithTimeout(parentCtx, cfg.JobTimeout)`
   - `reg.Set(jobID, cancel); defer reg.Remove(jobID)`
   - Build cmd:
     ```go
     cmd := exec.CommandContext(jobCtx, cfg.ClaudeBin,
       "-p", job.Command,
       "--output-format", "stream-json",
       "--permission-mode", cfg.ClaudePermissionMode,
       "--model", job.Model,
     )
     cmd.Dir = cfg.WorkDir
     cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
     cmd.Env = append(os.Environ()) // inherit for ~/.claude auth
     ```
   - Pipe stdout + stderr (keep simple `io.ReadAll` for phase 02; Phase 03 replaces with scanner).
   - `cmd.Start()` → on error, mark failed with error msg, return.
   - `cmd.Wait()` → capture exit code.
   - Determine final status:
     - `jobCtx.Err() == context.Canceled` → cancelled
     - `jobCtx.Err() == context.DeadlineExceeded` → failed, summary="timeout"
     - `cmd.ProcessState.ExitCode() != 0` → failed
     - else → done
   - Update job: status, exit_code, finished_at, summary (truncate to 500 chars for phase 02; Phase 03 parses properly).
6. Cancel implementation: on cancel request, kill entire process group:
   ```go
   if reg.Cancel(jobID) {
     // cancel() fires, CommandContext sends SIGKILL via cmd.Cancel, but we want SIGTERM first
   }
   ```
   Override with: set `cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }` and `cmd.WaitDelay = 5 * time.Second`.
7. Wire into `JobService.Enqueue`: insert pending row → `pool.Enqueue(job.ID)` → return.
8. `main.go`: `pool := worker.New(...); if err := pool.Recover(); err != nil { log.Fatal(err) }; pool.Start(rootCtx); router.Setup(...)`.

## Todo list
- [x] registry.go cancel map
- [x] pool.go struct + Start + Enqueue + Cancel
- [x] pool.Recover() boot recovery
- [x] runner.go exec w/ process group + ctx cancel
- [x] cmd.Cancel override for SIGTERM + WaitDelay
- [x] JobService wiring
- [x] main.go construct + start
- [x] Manual smoke: enqueue `echo hello`-style command, observe status transitions
- [x] Manual smoke: cancel running job, verify process dies within 5s
- [x] Manual smoke: kill -9 server mid-run, restart, verify recovery

## Success criteria
1. Submitting `{"command":"list files in current directory","model":"claude-sonnet-4-6"}` to `POST /api/jobs` returns 202 + job_id immediately.
2. Within seconds, job row shows status=running; after Claude exits, status=done.
3. Two concurrent submissions run in parallel when `WORKER_COUNT=2`; third queues until a worker frees.
4. `POST /api/jobs/:id/cancel` on a running job → subprocess terminates within 5s, status=cancelled.
5. Ctrl+C on server mid-job, restart → running jobs marked failed, pending jobs resume from queue.
6. Logs show `worker 0 started`, `worker 1 started`, per-job `running jobID=...`.

## Risks
| Risk | Mitigation |
|---|---|
| `claude` binary not authenticated for server user | README + preflight check at boot (optional): `claude --version` |
| Process group kill semantics on macOS dev vs Linux prod | Target Linux for acceptance; document macOS dev caveats |
| Long-running jobs blocking worker beyond `WaitDelay` | Use `JobTimeoutMin` as hard ceiling; subprocess force-killed on deadline |
| Channel fills if enqueue rate > worker rate | Make `jobs` channel buffered (e.g. 100); beyond that, return 503 from `POST /api/jobs` |
| Zombie grandchildren if Claude spawns own subprocesses | Process group kill covers entire tree; verify with `pstree` in smoke test |

## Security
- Subprocess inherits server env. Do NOT leak `JWT_SECRET` or DB creds via env into Claude subprocess — scrub `cmd.Env` to only pass what's needed (PATH, HOME, ANTHROPIC_API_KEY if set, CLAUDE_*).
- `WorkDir` is the trust boundary — Claude with bypassPermissions can read/write anything under it. Document this clearly.
- Cancel endpoint requires JWT.

## Next steps
Phase 03 — replace `io.ReadAll` stdout capture with stream-json line scanner, persist JobLog rows, enforce cost guard.
