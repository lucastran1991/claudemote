# Code Standards & Codebase Structure

## Codebase Layout

```
.
├── README.md                    # User-facing: overview, quick start, deploy
├── SYSTEM.md                    # DEPRECATED — system-architecture.md is current
├── system.cfg.json              # Single source of truth for non-secret config
├── Caddyfile.template           # Reverse proxy config template
├── ecosystem.config.cjs         # pm2 process manager config
├── start.sh                     # Deploy/bootstrap script (3-phase)
├── Makefile                     # Build targets
├── .gitignore
│
├── backend/                     # Go API server
│   ├── cmd/
│   │   ├── server/              # main entry point
│   │   └── create-admin/        # admin user creation utility
│   ├── internal/
│   │   ├── api/                 # HTTP handlers, routes
│   │   ├── jobs/                # Job queue, worker pool
│   │   ├── auth/                # JWT validation
│   │   ├── db/                  # SQLite schema + migrations
│   │   └── claude/              # Claude Code subprocess runner
│   ├── migrations/              # SQL migration files
│   ├── go.mod, go.sum
│   ├── server                   # compiled binary (gitignored)
│   ├── .env                     # secrets only (gitignored)
│   └── .env.example             # template for bootstrap
│
├── frontend/                    # Next.js UI
│   ├── app/
│   │   ├── (auth)/              # Login/logout pages
│   │   ├── (main)/              # Job submission, history
│   │   └── api/                 # NextAuth route handler
│   ├── public/                  # Static assets
│   ├── components/              # React components
│   ├── lib/                     # Utilities, API client
│   ├── .next/                   # production build (gitignored, recreated on deploy)
│   ├── .env.local               # secrets only (gitignored)
│   ├── .env.local.template      # template for bootstrap
│   ├── package.json
│   ├── pnpm-lock.yaml
│   └── next.config.ts
│
└── docs/                        # Project documentation
    ├── system-architecture.md   # Deployment & process architecture
    ├── deployment-guide.md      # Bootstrap + recurring deploy procedures
    ├── code-standards.md        # This file
    ├── project-overview-pdr.md  # Project overview & requirements
    └── codebase-summary.md      # Auto-generated codebase summary
```

## Configuration Management

### system.cfg.json (Committed)
Single source of truth for non-secret configuration. Read by:
- `start.sh` — generates Caddyfile, health checks
- `ecosystem.config.cjs` — injects into pm2 process env

**Never add secrets to system.cfg.json.** Secrets go in `.env` files.

**Example keys:**
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
Only secrets. Non-secret config belongs in `system.cfg.json`.

**backend/.env:**
```
JWT_SECRET=<random 64 hex chars>
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=<bcrypt hash>
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

## Backend (Go)

### Style & Conventions
- Follow standard Go conventions: `gofmt`, package-level comments, CamelCase for exported symbols
- Error handling: `if err != nil { return err }` (not panic)
- No globals except `init()` functions; pass dependencies as function args or struct fields
- Use interfaces for testability and decoupling

### Package Organization
```
backend/
├── cmd/
│   ├── server/              # HTTP server entry point
│   └── create-admin/        # Admin user creation utility
├── internal/
│   ├── api/                 # HTTP handlers + route setup
│   ├── jobs/                # Job queue, worker goroutines
│   ├── auth/                # JWT validation middleware
│   ├── db/                  # SQLite schema, migrations, queries
│   ├── claude/              # Claude subprocess runner
│   └── config/              # Env var + system.cfg.json parsing
└── migrations/              # SQL schema migrations (*.sql)
```

### Key Patterns

**Job queue:** In-memory Go channel. Workers pull and execute.

**SQLite:** Single-file database at `system.cfg.json:db_path`. Migrations run at startup.

**JWT auth:** Validated by middleware on each `/api/*` request (except `/api/auth/login`).

**SSE streaming:** `GET /api/jobs/:id/stream` writes JSON events as `data:` lines. Caddy forwards with `flush_interval -1` to disable buffering.

**Claude subprocess:** Runs `claude -p "<command>" --output-format json`, captures stdout + stderr, parses result.

### Testing
```bash
cd backend && go test ./...
```

### Build
```bash
cd backend && go build -o server ./cmd/server
```

## Frontend (Next.js)

### Tech Stack
- **Framework:** Next.js 14+ with App Router
- **Styling:** CSS modules or Tailwind (project choice)
- **Auth:** NextAuth v5 (session-based, JWT tokens stored in HTTP-only cookies)
- **API client:** Fetch API (no axios/got required)

### File Organization
```
frontend/
├── app/
│   ├── (auth)/              # /login, /logout routes
│   ├── (main)/              # / (home), protected pages
│   ├── api/
│   │   └── auth/[...nextauth]/ # NextAuth route handler
│   └── layout.tsx           # Root layout
├── components/              # Reusable React components
├── lib/
│   ├── api-client.ts        # Fetch wrapper, token management
│   └── auth.ts              # NextAuth config
└── public/                  # Static assets (logo, etc.)
```

### Key Patterns

**Server components:** Use by default. Move to client only when needed (useState, event listeners).

**API calls:** Use fetch in server components. In client components, wrap with try/catch and handle 401 (token expired).

**Token management:** NextAuth stores JWT in HTTP-only cookie. Include `credentials: 'include'` in fetch calls to pass it server-side.

**Form submission:** Use server actions (form action="/api/submit") instead of client-side fetch when possible.

### Testing
```bash
cd frontend && pnpm test
```

### Build
```bash
cd frontend && pnpm install && pnpm build
```

Production build output goes to `.next/` (gitignored, recreated on every deploy).

## Process Management (pm2)

**Config:** `ecosystem.config.cjs` at repo root.

**Two processes:**
- `claudemote-api` — Go binary, listens on `system.cfg.json:api.port`
- `claudemote-web` — Next.js, listens on `system.cfg.json:web.port`

**Non-secret config injected as env vars** from `system.cfg.json` by pm2 at startup.

**Secrets loaded from `.env` files** by the binaries themselves.

**Restart:** `pm2 reload ecosystem.config.cjs` picks up new binaries. Does NOT pick up `.env` changes (must be done manually or via full `./start.sh`).

## Deployment (start.sh & Caddy)

### start.sh
3-phase bootstrap script (Phase 1: discover, Phase 2: configure, Phase 3: build & start).

**Recurring deploy:** `./start.sh` rebuilds backend + frontend, reloads pm2, verifies health.

**Key tool dependency:** Uses `jq` to parse `system.cfg.json`. Install with `sudo apt install jq`.

### Caddyfile
HTTPS reverse proxy generated from `Caddyfile.template` by `start.sh` Phase 2.

**Routes:**
- `/api/jobs/*/stream` → Go backend (SSE, no buffering)
- `/api/auth/*` → Next.js (NextAuth)
- `/api/*` → Go backend
- Everything else → Next.js

**Not in git:** `Caddyfile` is generated at deploy time. Only commit `Caddyfile.template`.

## Development Workflow

### Local Development (No pm2)
```bash
# Terminal 1: Backend
cd backend && go run ./cmd/server

# Terminal 2: Frontend
cd frontend && pnpm dev
```

API: `http://localhost:8888`
Web: `http://localhost:3000` (or `8088` if you change the port in .env)

### Testing Before Deploy
```bash
# Build both
make build

# Run tests
make test

# Verify locally with pm2 (optional)
pm2 startOrReload ecosystem.config.cjs
pm2 logs
```

### Deploy
```bash
./start.sh          # Full deploy: build + reload
# or
make deploy         # Alias for ./start.sh
```

### Configuration Changes
1. Edit `system.cfg.json` or `.env` files
2. Run `./start.sh` (picks up all changes)

## Code Quality & Testing

### Requirements
- **No syntax errors** — code must compile/run
- **Reasonable test coverage** — unit tests for business logic
- **No secrets in code** — all credentials in `.env` or `system.cfg.json`
- **Error handling** — don't panic; return errors and log them

### Go Testing
```bash
cd backend && go test -v ./...
```

### Next.js Testing
```bash
cd frontend && pnpm test
```

### Linting (Optional)
- **Go:** `gofmt` (built-in)
- **Next.js:** `eslint` (optional, not enforced)

## Git Conventions

### Commit Messages
Use conventional commit format:
```
feat(module): add new feature
fix(auth): resolve token validation bug
docs(deploy): clarify bootstrap steps
refactor(jobs): simplify queue logic
test(worker): add concurrent job tests
```

### Branches
- `main` — production branch, merged via pull requests only
- Feature branches — `feat/description`, based on `main`
- Hotfixes — `hotfix/description`, based on `main`

### Pull Requests
1. Code review required
2. Tests must pass
3. No secrets in git history (use `.env` templates instead)

## Glossary

| Term | Meaning |
|------|---------|
| **system.cfg.json** | Single source of truth for non-secret config (hostname, ports, worker settings, job limits, paths). Committed to git. |
| **ecosystem.config.cjs** | pm2 config that reads system.cfg.json and injects values as env vars for Go and Next.js processes. |
| **Caddyfile** | HTTPS reverse proxy config (generated from Caddyfile.template by start.sh Phase 2). Not in git. |
| **WORK_DIR** | Filesystem path Claude Code operates inside (read from system.cfg.json, usually the repo root or a specific project). |
| **Worker** | Go goroutine that pulls jobs from the queue and runs Claude subprocesses. Count = system.cfg.json:worker.count. |
| **Job** | User-submitted Claude Code task. Stored in SQLite with status, logs, summary. |
| **SSE** | Server-Sent Events. Used for real-time job log streaming. Requires flush_interval -1 in Caddy. |

## Maintenance Notes

- **system.cfg.json is the source of truth** — all non-secret config flows from here
- **Caddyfile is generated** — edit Caddyfile.template, not Caddyfile directly
- **SYSTEM.md is deprecated** — refer to system-architecture.md instead
- **docker-compose.yml removed** — not used, Caddy + pm2 deployed directly on EC2
