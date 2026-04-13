# Codebase Summary

**Last Updated:** 2026-04-13  
**Project:** claudemote (Remote Claude Code Controller)

## Quick Overview

A full-stack web app (Go + Next.js) that submits and monitors Claude Code tasks on EC2. Configuration unified in a single `system.cfg.json` file (non-secret settings), with secrets in `.env` files (gitignored).

**Key Files:**
- `system.cfg.json` — Single source of truth for ports, hostname, worker settings, job limits, paths
- `ecosystem.config.cjs` — pm2 config that reads system.cfg.json and injects values as env vars
- `start.sh` — 3-phase deployment script (discover, configure, build & start)
- `Caddyfile.template` — Reverse proxy template (generates Caddyfile.template → /etc/caddy/Caddyfile)
- `backend/` — Go API server
- `frontend/` — Next.js UI

## Backend (Go)

### Entry Point
**`backend/cmd/server/main.go`** — Starts HTTP server, reads config from env vars (injected by pm2 from system.cfg.json), reads secrets from backend/.env.

### Key Packages

**`backend/internal/api/`**
- HTTP handlers and routes
- Implements job submission, history, streaming endpoints
- NextAuth/JWT middleware

**`backend/internal/jobs/`**
- In-memory job queue (Go channel)
- Worker pool (goroutines pull from queue)
- Subprocess execution (runs `claude -p` subprocesses)

**`backend/internal/db/`**
- SQLite schema and migrations
- CRUD queries (jobs, users)
- Migration runner (runs on startup)

**`backend/internal/auth/`**
- JWT validation
- Middleware for protected endpoints

**`backend/internal/claude/`**
- Claude Code subprocess runner
- Captures stdout/stderr
- Parses result JSON

**`backend/cmd/create-admin/`**
- Utility to create/reset admin user
- Run by `start.sh` Phase 3
- Reads ADMIN_USERNAME, ADMIN_PASSWORD from env

### Configuration Flow
```
system.cfg.json
    ↓ (read by start.sh via jq)
    ↓ (passed to pm2)
ecosystem.config.cjs
    ↓ (injects as env vars)
Go process
    ↓ (reads env)
backend/internal/config/
```

Non-secret config from environment: `PORT`, `WORKER_COUNT`, `WORK_DIR`, `CLAUDE_DEFAULT_MODEL`, `JOB_TIMEOUT_MIN`, `MAX_COST_PER_JOB_USD`, `JOB_LOG_RETENTION_DAYS`

Secrets from backend/.env: `JWT_SECRET`, `ADMIN_PASSWORD_HASH`, `CLAUDE_BIN`, `DB_PATH`

### Database Schema
```sql
-- Users (admin authentication)
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMP
);

-- Jobs (task history)
CREATE TABLE jobs (
  id TEXT PRIMARY KEY,
  command TEXT NOT NULL,
  model TEXT,
  status TEXT,           -- pending, running, done, failed, canceled
  summary TEXT,
  logs TEXT,
  created_at TIMESTAMP,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  cost_usd FLOAT
);
```

### Job Lifecycle
1. **POST /api/jobs** — Create job, insert to SQLite with status=pending
2. **Worker pulls from queue** — Runs `claude -p "<command>" --output-format json`
3. **Subprocess completes** — Worker parses result, updates SQLite (status=done|failed)
4. **GET /api/jobs/:id** — Returns job detail
5. **GET /api/jobs/:id/stream** — SSE stream of job events (real-time)

### Build
```bash
cd backend && go build -o server ./cmd/server
```

### Test
```bash
cd backend && go test ./...
```

## Frontend (Next.js)

### Entry Point
**`frontend/src/app/layout.tsx`** — Root layout, sets up NextAuth session provider.

### Key Routes

**`frontend/src/app/(auth)/`** — Login/logout pages
- `/login` — Username/password form
- `/logout` — Sign-out action

**`frontend/src/app/(dashboard)/`** — Protected pages (require auth)
- `/dashboard` — Job submission form + history table
- `/dashboard/jobs` — Job detail page
- `/dashboard/new` — New job submission page
- Layout enforces authentication redirect

**`frontend/src/app/api/auth/[...nextauth]/`** — NextAuth route handler
- Handles OAuth callbacks, session refresh, CSRF

### Configuration Flow
```
system.cfg.json (NOT read by frontend)
    ↓ (start.sh injects into pm2 env)
ecosystem.config.cjs
    ↓ (for web process, only uses web.port)
Next.js process
    ↓ (reads env from frontend/.env.local)
frontend/lib/auth.ts (NextAuth config)
```

Frontend env vars (from frontend/.env.local): `AUTH_SECRET`, `NEXTAUTH_SECRET`, `NEXTAUTH_URL`, `BACKEND_URL`

**Note:** Frontend does NOT read api.port, web.port, etc. from environment. It only reads NextAuth secrets and URLs.

### Key Components

**`frontend/src/components/`**
- Job submission form
- Job history table
- Status badge, spinner, log viewer

**`frontend/src/lib/`**
- `api-client.ts` — Fetch wrapper with token management
- `auth.ts` — NextAuth v5 config (session provider, credentials flow)

**`frontend/src/providers/`**
- React context providers (NextAuth session, theme, etc.)

**`frontend/src/types/`**
- TypeScript type definitions (Job, User, API responses)

### API Calls

**Client-side requests include JWT token** (stored in HTTP-only cookie by NextAuth). Fetch calls use `credentials: 'include'` to send cookie.

**Server-side requests** (in getServerSideProps, server actions) use Authorization header:
```typescript
Authorization: Bearer <jwt>
```

### Build
```bash
cd frontend && pnpm install && pnpm build
```

Output: `.next/` directory (gitignored, recreated on each deploy).

### Test
```bash
cd frontend && pnpm test
```

## Configuration Architecture

### system.cfg.json (Committed)
**Single source of truth for non-secret config.**

Read by:
1. `start.sh` (via jq) — Generates Caddyfile, extracts ports for health checks
2. `ecosystem.config.cjs` — Reads at Node.js module load time, injects into pm2 env

```json
{
  "hostname": "claudemote.example.com",
  "api": { "port": 8888 },
  "web": { "port": 8088 },
  "worker": {
    "count": 2,
    "model": "claude-sonnet-4-6",
    "permission_mode": "bypassPermissions"
  },
  "jobs": {
    "timeout_min": 30,
    "max_cost_usd": 1.0,
    "log_retention_days": 14
  },
  "db_path": "/var/lib/claudemote/claudemote.db",
  "work_dir": "/opt/atomiton/claudemote",
  "pm2_max_memory": "500M"
}
```

### .env Files (Gitignored)
Only secrets. Templates committed as `.env.example` and `.env.local.template`.

**backend/.env:**
```
JWT_SECRET=<random 64 hex>
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=<bcrypt>
CLAUDE_BIN=/path/to/claude
DB_PATH=/var/lib/claudemote/claudemote.db
```

**frontend/.env.local:**
```
AUTH_SECRET=<random 32+ base64>
NEXTAUTH_SECRET=<same as AUTH_SECRET>
NEXTAUTH_URL=https://claudemote.example.com
BACKEND_URL=http://localhost:8888
```

### ecosystem.config.cjs
Reads `system.cfg.json` at module load time. Injects values as env vars for pm2 processes:

**For claudemote-api (Go):**
- PORT (from api.port)
- WORKER_COUNT, CLAUDE_DEFAULT_MODEL, JOB_TIMEOUT_MIN, MAX_COST_PER_JOB_USD, JOB_LOG_RETENTION_DAYS (from job limits)
- WORK_DIR (from work_dir, or override with shell export)
- max_memory_restart (from pm2_max_memory)

**For claudemote-web (Next.js):**
- PORT (from web.port)
- BACKEND_URL, NEXTAUTH_URL, AUTH_SECRET (loaded from env by Next.js at startup)
- max_memory_restart (from pm2_max_memory)

### Caddyfile.template → Caddyfile
Generated by `start.sh` Phase 2. Uses sed to replace placeholders:

```bash
sed -e "s/{{HOSTNAME}}/${HOSTNAME}/g" \
    -e "s/{{API_PORT}}/${API_PORT}/g" \
    -e "s/{{WEB_PORT}}/${WEB_PORT}/g" \
    Caddyfile.template | sudo tee /etc/caddy/Caddyfile
```

**Not in git.** Only commit `Caddyfile.template`.

## Deployment Scripts

### start.sh
3-phase bootstrap + recurring deploy script.

**Phase 1: Discovery** (read-only, idempotent)
- Detect OS, install pnpm/Caddy if missing
- Verify Claude auth
- Prompt for WORK_DIR, hostname, URLs, admin credentials
- Verify all inputs

**Phase 2: Configuration** (writes only)
- Generate JWT_SECRET and AUTH_SECRET
- Write backend/.env (from template)
- Write frontend/.env.local (from template)
- Generate /etc/caddy/Caddyfile (from template)
- Create /var/lib/claudemote directory

**Phase 3: Build & Start** (destructive/stateful)
- Compile backend
- Build frontend
- Create admin user (runs backend create-admin utility)
- Reload pm2 (starts/restarts processes)
- Reload Caddy
- Health checks (curl /api/health, verify frontend responding)
- Optional smoke test (submit 1 trivial job, poll for completion)

**Recurring deploy (no flags):**
```bash
./start.sh  # builds both, reloads pm2
```

**Bootstrap:**
```bash
./start.sh --bootstrap          # First-run 3-phase setup
./start.sh --bootstrap --force  # Wipe env files, re-prompt
./start.sh --bootstrap --smoke-test  # Bootstrap + job verification
```

### Makefile
Common targets:

```bash
make dev        # Local development (Go + Next.js without pm2)
make build      # Compile backend + frontend
make test       # Run backend tests + frontend tests
make deploy     # Full deploy: build + reload
make reload     # pm2 reload (no rebuild)
make logs       # Tail pm2 logs
make create-admin  # Interactive admin user creation
```

## Process Architecture

### pm2 (ecosystem.config.cjs)
Two processes:

**claudemote-api** (Go)
- Binary: `backend/server`
- Listens: `system.cfg.json:api.port` (default 8888)
- Env: PORT, WORKER_COUNT, WORK_DIR, etc. (from system.cfg.json)
- Logs: `backend/logs/api-err.log`, `backend/logs/api-out.log`

**claudemote-web** (Next.js)
- Script: `pnpm start` in frontend/
- Listens: `system.cfg.json:web.port` (default 8088)
- Env: PORT, BACKEND_URL, NEXTAUTH_URL, AUTH_SECRET
- Logs: `frontend/logs/web-err.log`, `frontend/logs/web-out.log`

### Caddy (systemd)
Reverse proxy service:

```bash
sudo systemctl status caddy
sudo systemctl reload caddy
```

Config: `/etc/caddy/Caddyfile` (generated by start.sh)

Routes:
- `/api/jobs/*/stream` → Go backend (SSE, flush_interval -1)
- `/api/auth/*` → Next.js (NextAuth)
- `/api/*` → Go backend
- Everything else → Next.js

## Data Flow: Job Submission

1. User fills form in Next.js UI → `POST /api/jobs`
2. Frontend includes JWT token (in HTTP-only cookie)
3. Caddy routes to Go backend (reverse proxy)
4. Go API handler validates JWT, inserts job to SQLite (status=pending)
5. Returns job_id immediately (async)
6. Worker goroutine pulls from queue, executes subprocess
7. Subprocess completes, worker updates SQLite (status=done|failed)
8. Frontend polls `GET /api/jobs/$id` (or streams `GET /api/jobs/$id/stream`)
9. User sees progress in real-time

## Secrets Management

### Committed (safe)
- `system.cfg.json` — Non-secret config
- `.env.example` — Template (placeholder secrets)
- `.env.local.template` — Template (placeholder secrets)

### NOT Committed (dangerous if leaked)
- `backend/.env` — JWT_SECRET, admin password hash, CLAUDE_BIN
- `frontend/.env.local` — AUTH_SECRET, NEXTAUTH_SECRET
- `Caddyfile` — Generated from template, may contain hostname (public but generated)

### Rotation
- **JWT_SECRET** — Change annually. Invalidates all issued tokens. Requires `backend/.env` edit + `./start.sh`
- **AUTH_SECRET** — Change annually. Invalidates all NextAuth sessions. Requires `frontend/.env.local` edit + `./start.sh`
- **Admin password** — Change via `make create-admin` or login form (post-v1.0)
- **Claude auth** — ANTHROPIC_API_KEY or OAuth creds in ~/.claude/.credentials.json (external, not in claudemote)

## Error Handling

**Backend:** Functions return `error`. Logged with context. HTTP endpoints return 4xx/5xx with JSON error.

**Frontend:** try/catch on fetch calls. Handle 401 (token expired) by redirecting to login.

**Job execution:** Stderr captured and stored in job summary. If subprocess fails, job marked as failed (not panic).

## Testing Strategy

**Backend:**
- Unit tests in `*_test.go` files
- Mocks for external services (SQLite optional, use in-memory for tests)
- Run: `go test ./...`

**Frontend:**
- Jest for unit tests
- Optional: Playwright for E2E
- Run: `pnpm test`

## Performance Notes

- **Job queue:** In-memory Go channel (no persistence, jobs lost on restart)
- **Database:** SQLite (single file, ~100 jobs/sec write throughput)
- **Concurrent jobs:** Limited by `system.cfg.json:worker.count` (default 2)
- **SSE latency:** Sub-second (flush_interval -1 disables buffering)
- **Memory:** pm2 restarts process if exceeds `system.cfg.json:pm2_max_memory` (default 500M)

## File Structure Summary

```
.
├── README.md                    # User-facing overview
├── SYSTEM.md                    # DEPRECATED (use docs/system-architecture.md)
├── Makefile                     # Build targets
├── system.cfg.json              # Single source of truth (non-secret config)
├── Caddyfile.template           # Reverse proxy template
├── ecosystem.config.cjs         # pm2 config (reads system.cfg.json)
├── start.sh                     # Deploy/bootstrap script
│
├── backend/                     # Go API
│   ├── cmd/server/main.go
│   ├── cmd/create-admin/
│   ├── internal/{api,jobs,db,auth,claude,config}
│   ├── migrations/
│   ├── go.mod, go.sum
│   ├── server                   # compiled binary
│   ├── .env                     # secrets (gitignored)
│   ├── .env.example             # template
│   └── logs/
│
├── frontend/                    # Next.js UI
│   ├── app/(auth)/, app/(main)/, app/api/auth
│   ├── components/, lib/
│   ├── public/
│   ├── .next/                   # build (gitignored)
│   ├── .env.local               # secrets (gitignored)
│   ├── .env.local.template      # template
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── next.config.ts
│   └── logs/
│
└── docs/                        # Project documentation
    ├── system-architecture.md
    ├── deployment-guide.md
    ├── code-standards.md
    ├── project-overview-pdr.md
    └── codebase-summary.md      # This file
```

## Key Design Patterns

1. **Configuration as code:** system.cfg.json committed, .env files gitignored
2. **Process manager abstraction:** pm2 reads system.cfg.json, injects as env
3. **Secrets isolation:** Separated into .env files, loaded by binaries (not in git)
4. **Single source of truth:** All non-secret config flows from system.cfg.json
5. **Zero config ops:** Caddy auto Let's Encrypt, pm2 auto-restart, migrations auto-run

## Glossary

| Term | Definition |
|------|-----------|
| **WORK_DIR** | Filesystem path where Claude Code operates (read from system.cfg.json) |
| **Worker** | Go goroutine that pulls jobs from queue and runs Claude subprocesses |
| **Job** | User-submitted Claude Code task (stored in SQLite) |
| **SSE** | Server-Sent Events (real-time log streaming, requires flush_interval -1) |
| **NextAuth v5** | Session-based auth (JWT in HTTP-only cookie) |
| **ecosystem.config.cjs** | pm2 config file (Node.js module that reads system.cfg.json) |
| **Caddyfile** | Caddy reverse proxy config (generated from Caddyfile.template) |

---

**For more detailed information, see:**
- `deployment-guide.md` — Bootstrap and deploy procedures
- `system-architecture.md` — Architecture, processes, configuration management
- `code-standards.md` — Codebase structure, conventions, development workflow
- `project-overview-pdr.md` — Project overview, requirements, design decisions
