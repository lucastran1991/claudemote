# Adversarial Code Review — claudemote v1

**Date:** 2026-04-13
**Reviewer:** code-reviewer subagent
**Scope:** production readiness, NOT style audit
**Verdict up front:** **SHIP WITH BLOCKERS FIXED** — 2 critical bugs (data race in LogWriter; terminal job log viewer broken); 1 likely SSE-latency issue in Caddy; rest is fine.

## TL;DR

| # | Severity | Finding | File |
|---|---|---|---|
| C1 | CRITICAL | Data race on `LogWriter.buf` / `seq` — stdout + stderr goroutines call `Append` without a mutex | `backend/internal/worker/log_writer.go:36`, `runner.go:84-98` |
| C2 | CRITICAL | Terminal job log viewer shows empty output — frontend skips SSE fetch entirely when `isTerminal` | `frontend/src/components/jobs/job-log-viewer.tsx:102-111` |
| I1 | IMPORTANT | Caddy `encode gzip` applies to SSE route — gzip buffers events, kills live latency despite `flush_interval -1` | `Caddyfile:14,20-28` |
| I2 | IMPORTANT | History-vs-live replay gap — unflushed LogWriter lines (up to 50) are lost when client subscribes mid-job | `backend/internal/handler/stream_handler.go:68-87` + `log_writer.go:10` |
| I3 | IMPORTANT | No NEXTAUTH_SECRET / AUTH_SECRET set anywhere — NextAuth v5 will refuse to sign in prod | `ecosystem.config.cjs:32-46`, `frontend/src/lib/auth.ts` |

---

## Critical findings

### C1 — Data race on `LogWriter` (concurrent stdout + stderr appenders)
**Evidence:**
- `backend/internal/worker/runner.go:83-96` — a goroutine drains stderr and calls `lw.Append("stderr", line)`.
- `backend/internal/worker/runner.go:98` — the main goroutine calls `ParseStream(stdoutPipe, lw)` which internally calls `lw.Append("stdout", line)` (`stream_parser.go:34`).
- `backend/internal/worker/log_writer.go:15-57` — `LogWriter` has no mutex. `Append` mutates `w.seq`, `w.buf`, and calls `w.repo.CreateBatch` on overflow, all without synchronization.

**Failure mode:**
- Lost/duplicate seq numbers (two goroutines read-modify-write `w.seq` concurrently).
- Torn `w.buf` append (slice header race under append-with-capacity-exceeded).
- `CreateBatch` called concurrently with another `Append`, which may mutate the buffer mid-insert.
- `Publish(jobID, seq, line)` may emit out-of-order / duplicate seqs to SSE subscribers, which breaks `Last-Event-ID` resume correctness.
- Undefined behaviour under `-race` detector; will manifest as random test failures and subtle production data corruption.

**Fix:** Add a `sync.Mutex` to `LogWriter` and lock around the entire `Append` body (the entire flush is cheap — batch insert is already async-decoupled at seq 50). Alternative: funnel stderr through a channel into a single consumer that owns the writer.

**Verdict:** ACCEPT — blocker.

---

### C2 — Terminal jobs show empty log viewer
**Evidence:**
- `frontend/src/components/jobs/job-log-viewer.tsx:102-111`:
  ```ts
  useEffect(() => {
    if (!token || isTerminal) return   // ← early return skips everything
    connect()
    ...
  }, [token, jobId, isTerminal, connect])
  ```
- `frontend/src/app/(dashboard)/dashboard/jobs/[id]/page.tsx:55-68` passes `isTerminal={true}` for done/failed/cancelled jobs.
- Backend `stream_handler.go:69-84` explicitly serves full history and returns cleanly for terminal jobs, so a one-shot fetch would work — the frontend simply never calls it.
- Result: completed jobs show `"No output recorded."` (line 163), regardless of actual log content. The job-history feature is broken.

**Failure mode:** Primary UX regression — user runs a job, it completes, clicks into detail, sees nothing. Blocks the core "review past runs" use case.

**Fix:** Remove the `isTerminal` short-circuit. Connect the EventSource unconditionally; the backend already returns and closes the stream immediately for terminal jobs (replays full history, then exits).

**Verdict:** ACCEPT — blocker.

---

## Important findings

### I1 — `encode gzip` on SSE route
**Evidence:** `Caddyfile:14,20-28` — `encode gzip` is set at the site level before the `@stream` matcher. Caddy's encode module compresses `text/*` content types, including `text/event-stream`. `flush_interval -1` flushes the reverse-proxy body writer, but the gzip writer sits **after** the proxy, and gzip buffers internally until it has enough data to emit a compressed chunk.

**Failure mode:** SSE events delayed or batched. Observed symptom: UI sits on "Waiting for output…" for seconds/minutes, then flushes many lines at once. Bypassing proxy buffering with `X-Accel-Buffering: no` (set in `stream_handler.go:59`) does not help because the gzip is Caddy's own middleware, not the upstream's.

**Fix:** Exclude the stream path from compression:
```caddy
encode gzip {
  not path /api/jobs/*/stream
}
```
Or inside the `@stream handle` block, disable via an inner `encode` override.

**Verdict:** ACCEPT — likely real, should be fixed pre-ship. Verify empirically after fix.

---

### I2 — History/live replay gap (lost in-flight log lines)
**Evidence:**
- `backend/internal/worker/log_writer.go:10,52` — `logFlushThreshold = 50`. Lines are batched and flushed to DB only every 50 lines (or at job end).
- `backend/internal/worker/log_writer.go:46-49` — `Publish` fires synchronously inside `Append`, BEFORE the buffer flush.
- `backend/internal/handler/stream_handler.go:69` — `ListAfterSeq` queries DB (only flushed rows visible).
- `stream_handler.go:87` — `hub.Subscribe` registers only **after** the history query returns.

**Failure mode:** Timeline for mid-job connect:
1. Client fetches history → DB has seqs 0..49 (flushed), worker is at seq 75 in-memory.
2. Handler returns history (0..49), then calls `Subscribe`.
3. Worker publishes seq 76 → delivered to subscriber (OK).
4. Seqs 50..75 were Publish'd before step 2 completed — no subscriber existed; they are in LogWriter buffer but not in DB. **Permanently lost from the client's view** until the final Flush on job completion writes them to DB (client won't re-query).

Effect: client observes a gap of up to 49 lines whenever connecting mid-job. For short jobs that never hit a 50-line flush, `eventCount%20` status re-check in the handler may not trigger either, but the late terminal flush + job status change → frontend detects terminal via `useJob` poll → next reload would show full logs only if we fix C2.

**Fix options:**
1. Subscribe **before** querying history, buffer events into a local slice, then dedupe against history by seq. Standard pattern.
2. Reduce flush threshold to 1 (just wastes writes).
3. On SSE disconnect / job completion, refetch history on the client to reconcile.

**Verdict:** ACCEPT — important, noticeable, should-fix.

---

### I3 — Missing NEXTAUTH_SECRET / AUTH_SECRET
**Evidence:** `ecosystem.config.cjs:36-42` — `claudemote-web` env block has only `PORT`, `BACKEND_URL`, `NEXT_PUBLIC_BACKEND_URL`. No `AUTH_SECRET` / `NEXTAUTH_SECRET`. `frontend/src/lib/auth.ts` does not pass a `secret` option. NextAuth v5 in production **requires** `AUTH_SECRET`; it will throw `MissingSecret` on first request, or log a hard warning if not in strict mode.

**Failure mode:** Web process crashes on first login attempt in prod. May be recovered via `.env.local` (not visible to this reviewer), but pm2 ecosystem is the single source of truth per the deploy docs — not guaranteed.

**Fix:** Either add `AUTH_SECRET` to `frontend/.env.local.template` and document, or add it to the pm2 env block pulled from a shell `.env`. Document in README.

**Verdict:** ACCEPT — pre-deploy fix.

---

### I4 — Login endpoint has no rate limiting
**Evidence:** `backend/internal/handler/auth_handler.go:30-48`. No middleware, no counter. `router.go:35` mounts it publicly.

**Failure mode:** Brute force against `POST /api/auth/login`. bcrypt cost 10 → ~100 hashes/sec on a small VM, attacker runs a dictionary through directly. Single admin account → single point of failure.

**Fix:** Add simple IP-based rate limiter (e.g., `github.com/ulule/limiter` or a lightweight in-proc tokenbucket). 5 attempts / min / IP is standard.

**Verdict:** ACCEPT — should-fix, not a pre-ship blocker if you trust the network layer, but trivial to add.

---

### I5 — JWT_SECRET has no minimum-length validation
**Evidence:** `backend/internal/config/config.go:60-78` only checks non-empty. A one-byte secret passes `validate()`.

**Failure mode:** Weak secret → HS256 brute-forceable (GPU-feasible under ~20 chars). Ops mistake, not an attack.

**Fix:** Enforce `len(c.JWTSecret) >= 32` in `validate()`.

**Verdict:** ACCEPT — trivial safety net.

---

### I6 — Orphan "pending" rows on queue-full enqueue
**Evidence:** `backend/internal/service/job_service.go:74-78` — `Enqueue` creates DB row first, then pushes to channel. If `pool.Enqueue` fails with `ErrQueueFull`, the row remains in the DB with status `pending` but no worker will ever pick it up (until the next server restart when `Recover()` re-enqueues it).

**Failure mode:** User sees 503, resubmits → duplicate job rows pile up. On next boot `Recover` replays them all → bill spike.

**Fix:** On `pool.Enqueue` failure, delete the just-created row (or mark it `failed` with summary `queue-full`). Matters only under sustained load > 100 pending; noted for v2.

**Verdict:** ACCEPT — defensible as DEFER, but easy to fix now.

---

### I7 — Command length not validated server-side
**Evidence:** `backend/internal/handler/job_handler.go:13-16` — `Command string` with only `binding:"required"`. A 5 MB prompt body is accepted and persisted.

**Failure mode:** DoS via huge POST bodies. Gin has a default 32 MiB max body, no specific validation. Combined with lack of rate limit → trivial to fill disk.

**Fix:** Add `binding:"max=16000"` or similar, or explicit check in handler. Also set `r.MaxMultipartMemory` / use `maxBytesHandler` wrapper at router.

**Verdict:** ACCEPT — should-fix.

---

## Observations (DEFER / known risk)

| # | Area | Note |
|---|---|---|
| O1 | Trust model | `bypassPermissions` + fixed `WORK_DIR` — Claude can traverse/write anywhere the uid can. Acknowledged in brainstorm. |
| O2 | Token in URL | `?token=` appears in access logs + browser history (`jwt_auth_with_query.go:18`). Documented. Mitigation would require SSE POST (not standard) or cookie auth. |
| O3 | Job list endpoint | `repository.List()` has no pagination — will scale poorly past ~10k rows. Phase doc already notes "add pagination". |
| O4 | Stream terminal re-check | `stream_handler.go:110` only re-checks status every 20 events — a job producing exactly 19 final lines and then finishing would never trigger the close path until the next event. Benign; flushing is correct, stream stays open but idle until client disconnect. Flag for v2. |
| O5 | Server-shutdown cancellation | On SIGTERM the root ctx cancels all job ctxs → `resolveOutcome` classifies them as `"cancelled"` rather than `"interrupted"` or leaving them `"running"` for Recover. Minor semantic oddity (`runner.go:128-133`). |
| O6 | 4 MB scanner ceiling | `stream_parser.go:28` — a single >4 MB stream-json line kills the job with `scan error`. Unlikely with Claude Code, but uncapped upstream. |
| O7 | Post-hoc cost guard | `runner.go:148-149` — cost check only runs if Claude emits a final `result` event. A runaway that never emits one is capped only by `JOB_TIMEOUT_MIN` wall-clock. Documented. |
| O8 | Caddy `path /api/jobs/*/stream` matcher | Caddy uses glob-style matching where `*` matches any chars including `/`. UUID path `/api/jobs/abc-def/stream` matches. OK. |
| O9 | `isAllowedEnvKey` permissive prefix | Any `CLAUDE_*` env var is forwarded (`runner_helpers.go:88-94`). No current secret follows that pattern, but worth documenting: don't name a secret `CLAUDE_FOO`. |
| O10 | `create-admin` password via `-password` flag | Visible in `ps auxww`. Use `-` / stdin prompt for production. Minor. |
| O11 | NextAuth session TTL ≠ backend JWT TTL | Backend token expires after 24 h but NextAuth session cookie default is 30 d. Client will silently get 401s with no UX (no refresh flow, no redirect-to-login). Fix later: detect 401 → `signIn()`. |
| O12 | Cancel semantics for partially-cancelled pending job | DB race: worker dequeued but not yet updated status to running, user cancels → `Cancel` writes `cancelled`, worker then transitions to `running`. `runner.go:34-37` guards by checking `Status != "pending"` — but a small window exists between `Status = "running"; Update(job)` and the guard in the next iteration. Under normal contention this is fine; SQLite WAL gives read consistency. |

---

## False positives / rejected findings

| Claim | Rejection reason |
|---|---|
| JWT `alg: none` risk | `token.go:35-37` type-asserts `*jwt.SigningMethodHMAC` — rejects non-HMAC algs including `none`. Correct. |
| Query-token middleware leaks onto other routes | `router.go:49-53` — `JWTAuthWithQuery` is scoped to a dedicated group containing only the stream route. Correct isolation. |
| SSE hub send-on-closed-channel race | `sse/hub.go:43-50` — `unsub` takes the write lock (exclusive of all Publish RLocks), deletes the entry, then closes. Any Publish that acquires RLock after unsub doesn't see the channel. Safe. |
| SQL injection in `ListAfterSeq` | `job_log_repository.go:46-53` — parameterized via GORM `?` placeholders. Safe. |
| XSS in log viewer | `job-log-viewer.tsx:161-168` — lines rendered as React text children inside `<pre>`, not `dangerouslySetInnerHTML`. React auto-escapes. Safe. |
| Recover() ordering | `main.go:54-62` — `Recover()` runs before `Start()`. Recover enqueues into the buffered channel, Start then drains. Correct (note: caps at buffer size 100; see O4 variant). |
| Process group cancellation broken on macOS | `runner_helpers.go:35-45` — `Setpgid: true` + `syscall.Kill(-pid, SIGTERM)` is the portable Unix idiom. Works on Darwin and Linux identically. |
| Env scrub missing | `runner_helpers.go:74-94` — JWT_SECRET, ADMIN_*, DB_PATH, CORS_ORIGIN are all excluded. Only PATH, HOME, ANTHROPIC_API_KEY, CLAUDE_* pass. Correct (see O9 caveat). |
| Registry Set/Cancel/Remove race | `registry.go` — all methods hold the same mutex; `Cancel` may double-fire with deferred `Remove` but the second delete is a no-op on missing key. Safe. |

---

## Unresolved questions

1. Is `AUTH_SECRET` set in `frontend/.env.local` (not visible in the committed template)? If yes, I3 is downgraded to documentation-only.
2. Does Caddy's `encode gzip` in practice buffer `text/event-stream` responses? Empirical test (curl a streamed endpoint through Caddy, observe chunk boundaries) is the only way to confirm I1; my claim is based on module behaviour, not a reproduction.
3. What is the expected max payload size for a user prompt? Needed to pick a sane cap for I7.
4. Is there a plan to expose per-user scoping later, or is admin-only the permanent model? `Authorization` check is "valid JWT", not "owns this job" — any authenticated user can read/cancel any job. Fine for v1 single-admin, but a risk if a second account is ever added.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Ship-blocker count is low (2 critical, fixable in <1 h each). C1 is a real Go data race — add `sync.Mutex` to `LogWriter`. C2 is a one-line frontend fix. After those, the Caddy gzip and NextAuth secret issues should also be settled pre-deploy. Everything else is important-or-defer with acceptable workarounds.
**Concerns/Blockers:** C1 LogWriter data race; C2 terminal-job log viewer empty; I1 Caddy gzip buffers SSE; I3 missing AUTH_SECRET in deploy config.
