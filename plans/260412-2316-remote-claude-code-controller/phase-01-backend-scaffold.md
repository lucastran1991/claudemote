# Phase 01 — Backend Scaffold (Gin + GORM + SQLite + JWT)

## Context
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md
- Reference: /Users/mac/studio/playground/backend

## Overview
**Priority:** P0
**Status:** pending
**Effort:** M

Scaffold Go backend mirroring the playground layout. No business logic yet beyond login + health + empty Job CRUD. Sets the skeleton every later phase builds on.

## Key insights
- Playground already uses Gin + GORM + glebarez/sqlite + JWT v5 + godotenv — lift all patterns wholesale.
- Migrations are embedded SQL files tracked in `schema_migrations` table — same pattern here.
- DI pattern: `handler ← service ← repository ← *gorm.DB`. No global state.

## Requirements
**Functional**
- `POST /api/auth/login` → JWT on valid admin creds (bcrypt check).
- `GET /api/health` → `{ok: true}`.
- `GET /api/jobs` + `POST /api/jobs` + `GET /api/jobs/:id` + `POST /api/jobs/:id/cancel` — stubbed handlers returning empty/placeholder data.
- JWT middleware on all `/api/*` except `/api/auth/login` and `/api/health`.

**Non-functional**
- `go build ./cmd/server` clean, no warnings.
- Config loaded from env + `.env` file; fails fast on missing required vars.
- SQLite file path configurable; auto-created + migrated on boot.

## Architecture
```
backend/
  cmd/server/main.go              # boot: config → db → migrate → wire → gin.Run
  internal/
    config/config.go              # env struct + validation
    database/database.go          # gorm open + migrate
    database/migrations/*.sql     # embedded via go:embed
    model/job.go                  # Job, JobLog gorm structs
    model/user.go                 # admin User
    repository/job_repository.go
    repository/job_log_repository.go
    repository/user_repository.go
    service/auth_service.go
    service/job_service.go        # enqueue/list/get (no worker wiring yet)
    handler/auth_handler.go
    handler/job_handler.go        # stub endpoints
    handler/health_handler.go
    middleware/jwt_auth.go
    router/router.go              # Setup(db, cfg, services) *gin.Engine
  pkg/response/response.go        # JSON helpers (OK, Err, Paginated)
  .env.example
  go.mod
```

## Related code files
**Create:** all files above.
**Modify:** none (new project).
**Delete:** none.

## Implementation steps
1. `cd /Users/mac/studio/claudemote && mkdir backend && cd backend && go mod init github.com/<user>/claudemote/backend`.
2. `go get` deps: `github.com/gin-gonic/gin`, `gorm.io/gorm`, `github.com/glebarez/sqlite`, `github.com/golang-jwt/jwt/v5`, `github.com/joho/godotenv`, `github.com/google/uuid`, `golang.org/x/crypto/bcrypt`.
3. `internal/config/config.go`: load env via godotenv, `Config` struct with fields:
   - `Port` (default 8080)
   - `WorkerCount` (default 2)
   - `WorkDir` (required)
   - `ClaudeBin` (default `/usr/local/bin/claude`)
   - `ClaudeDefaultModel` (default `claude-sonnet-4-6`)
   - `ClaudePermissionMode` (default `bypassPermissions`)
   - `JobTimeoutMin` (default 30)
   - `MaxCostPerJobUSD` (default 1.0)
   - `JobLogRetentionDays` (default 14)
   - `DBPath` (default `./claudemote.db`)
   - `JWTSecret` (required)
   - `AdminUsername` (required)
   - `AdminPasswordHash` (required, bcrypt)
   - Validate all required on startup, panic with clear message on miss.
4. `internal/database/database.go`: open sqlite via gorm, run embedded migrations.
5. Migrations (three files in `migrations/`):
   - `001_create_users.up.sql` — users(id, username unique, password_hash, created_at).
   - `002_create_jobs.up.sql` — jobs(id uuid pk, command text, model text, status text, exit_code int nullable, summary text, session_id text, duration_ms int, total_cost_usd real, num_turns int, is_error bool, stop_reason text, created_at, started_at nullable, finished_at nullable).
   - `003_create_job_logs.up.sql` — job_logs(id pk, job_id fk, seq int, stream text, line text, created_at). Index on `(job_id, seq)`.
6. `internal/model/*.go` — GORM structs matching migration columns.
7. Repositories: basic `Create`, `FindByID`, `List`, `Update`, `Delete` as needed. `JobRepository.ListRecoverable()` returns `WHERE status IN ('pending','running')` for boot recovery (used in Phase 02).
8. `internal/middleware/jwt_auth.go` — bearer parser, validates HS256 with `JWTSecret`, puts `user_id` in ctx.
9. `internal/service/auth_service.go` — `Login(username, password)` → JWT or error. Uses bcrypt compare.
10. `internal/handler/auth_handler.go` — `POST /api/auth/login` wires service.
11. `internal/handler/job_handler.go` — stubs that return empty/`nil` but wire through service/repo.
12. `internal/router/router.go` — `Setup()` wires everything, returns `*gin.Engine`.
13. `cmd/server/main.go` — boot sequence.
14. `.env.example` with all vars documented.
15. Seed admin user: create a `cmd/create-admin/main.go` CLI that reads username + bcrypt-hashed password from stdin/flags and inserts into users table. (Playground has this pattern — lift it.)
16. `go build ./...` → clean.

## Todo list
- [x] go mod init + deps fetched
- [x] config loader + validation
- [x] database + embedded migrations
- [x] models (Job, JobLog, User)
- [x] repositories
- [x] auth service + handler
- [x] JWT middleware
- [x] job handler stubs
- [x] router wiring
- [x] main.go boot
- [x] cmd/create-admin CLI
- [x] .env.example
- [x] `go build ./...` clean
- [x] Manual smoke: start server, login, 401 on protected route without token

## Success criteria
1. `go build ./cmd/server && ./server` starts on configured port, creates SQLite file, runs migrations.
2. `GET /api/health` returns 200 `{ok: true}`.
3. `POST /api/auth/login` with correct admin creds returns a JWT.
4. `GET /api/jobs` without token returns 401; with valid JWT returns empty array.
5. Missing required env var (e.g. `WORK_DIR`) fails boot with readable error.

## Risks
| Risk | Mitigation |
|---|---|
| Version drift from playground | Pin same go.mod versions as playground where possible |
| SQLite file perms in prod | Document `chmod 600` on DB path in phase 07 |
| bcrypt cost too high blocks login | Use cost 10 (default) |

## Security
- Passwords bcrypt hashed, never stored plain.
- JWT secret validated non-empty at boot, fails loud.
- Admin creation out-of-band via CLI only — no signup endpoint.
- JWT TTL: 24h initially; refresh flow out of v1.

## Next steps
Phase 02 — plug worker pool into `JobService.Enqueue` and add subprocess runner.
