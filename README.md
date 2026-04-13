# claudemote

Remote Claude Code controller — submit and monitor Claude Code tasks on an EC2 instance from any browser or iOS device.

## Stack

- **Backend**: Go — HTTP API, job queue, SSE streaming, SQLite
- **Frontend**: Next.js — job submission UI, live log streaming
- **Proxy**: Caddy — HTTPS termination, SSE passthrough, reverse proxy
- **Process manager**: pm2 — two processes (`claudemote-api`, `claudemote-web`)

## Configuration

Non-secret config (ports, hostname, worker count, model, job limits, paths) lives in **`system.cfg.json`** at the repo root — this file is committed and is the single source of truth for runtime tunables.

Secrets (JWT signing key, admin credentials, NextAuth secret) stay in `.env` files that are gitignored.

`ecosystem.config.cjs` reads `system.cfg.json` at pm2 startup and injects all values as env vars into the Go backend and Next.js processes. To change a port or any non-secret setting: edit `system.cfg.json`, then run `./start.sh`.

## Local Development

```bash
cp backend/.env.example backend/.env        # fill in JWT_SECRET, ADMIN_PASSWORD_HASH
cp frontend/.env.local.template frontend/.env.local   # fill in AUTH_SECRET
./start.sh                                  # builds + runs in foreground (Ctrl+C to stop)
```

API: `http://localhost:8888` | Web: `http://localhost:8088`

## Deploy (EC2)

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | `sudo apt install golang-go` or [go.dev](https://go.dev/dl/) |
| Node.js | 20+ | [nodesource](https://github.com/nodesource/distributions) |
| pnpm | latest | `npm install -g pnpm` |
| pm2 | latest | `npm install -g pm2` |
| Caddy | 2.x | `sudo apt install caddy` (Caddy apt repo) |
| Claude Code CLI | latest | [docs.anthropic.com](https://docs.anthropic.com/claude-code) — must be authenticated |

EC2 security group: open ports **80** and **443** (HTTP/HTTPS) inbound.

### One-time setup (first deploy only)

```bash
# 1. Point DNS A record → EC2 public IP before this step (Caddy needs it for TLS)

# 2. Install and configure Caddy
sudo apt install caddy
# Edit Caddyfile: replace claudemote.example.com with your actual domain
nano Caddyfile
sudo cp Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy

# 3. Create env files from templates and fill in secrets
cp backend/.env.example backend/.env
#    Edit backend/.env — set JWT_SECRET, ADMIN_USERNAME, ADMIN_PASSWORD_HASH, CLAUDE_BIN
cp frontend/.env.local.template frontend/.env.local
#    Edit frontend/.env.local — set AUTH_SECRET and NEXTAUTH_SECRET

# 3a. Verify system.cfg.json has the correct hostname, ports, and WORK_DIR for this host

# 4. Configure pm2 to survive reboots
pm2 startup systemd -u $USER --hp /home/$USER
#    Run the printed sudo command, then:
pm2 save

# 5. Create admin user
make create-admin

# 6. First boot
./start.sh
```

### Deploy loop (after every code change)

```bash
git pull
./start.sh --prod   # rebuilds backend + frontend, reloads pm2
```

Or, if you only want to reload without a full rebuild:

```bash
make reload
```

### Make targets

| Target | Description |
|--------|-------------|
| `make dev` | Start both servers locally (no pm2) |
| `make build` | Compile Go binary + Next.js production build |
| `make test` | Run backend Go tests + frontend tests |
| `make migrate-up` | Apply DB migrations |
| `make create-admin` | Create initial admin user |
| `make deploy` | `build` + `reload` |
| `make reload` | `pm2 reload ecosystem.config.cjs` |
| `make logs` | Tail all pm2 logs |
| `make clean` | Remove `backend/server` and `frontend/.next` |

### Environment variables

Secrets only — all other config is in `system.cfg.json`.

**`backend/.env`** (never commit this file):

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Random 32+ char string for signing tokens — `openssl rand -hex 32` |
| `ADMIN_USERNAME` | Admin login username |
| `ADMIN_PASSWORD_HASH` | bcrypt hash of admin password — set by `make create-admin` |
| `CLAUDE_BIN` | Absolute path to the `claude` binary — auto-detected by `./start.sh --bootstrap` |

**`frontend/.env.local`** (never commit this file):

| Variable | Description |
|----------|-------------|
| `AUTH_SECRET` | Random 32+ char string for NextAuth v5 JWT signing — `openssl rand -base64 32` |
| `NEXTAUTH_SECRET` | Alias kept for NextAuth v4 compatibility — set to same value as `AUTH_SECRET` |

**`system.cfg.json`** (committed, non-secret config):

| Key | Description |
|-----|-------------|
| `hostname` | Public domain name (used by Caddy and NextAuth URL) |
| `api.port` | Go API listen port (default: 8888) |
| `web.port` | Next.js listen port (default: 8088) |
| `worker.count` | Concurrent Claude Code subprocesses |
| `worker.model` | Claude model used when job doesn't specify one |
| `worker.permission_mode` | Claude permission mode (e.g. `bypassPermissions`) |
| `jobs.timeout_min` | Wall-clock timeout per job in minutes |
| `jobs.max_cost_usd` | Max API spend per job before auto-cancel |
| `jobs.log_retention_days` | Days to retain job log rows after completion |
| `db_path` | SQLite file path |
| `work_dir` | Repo path Claude Code operates inside |

### SSE smoke test

```bash
TOKEN=$(curl -s -X POST https://your-domain/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"..."}' | jq -r .data.token)

JOB_ID=$(curl -s -X POST https://your-domain/api/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command":"list files","model":"claude-sonnet-4-6"}' | jq -r .data.id)

# Should stream lines in real time with no buffering delay
curl -N -H "Authorization: Bearer $TOKEN" \
  "https://your-domain/api/jobs/$JOB_ID/stream"
```

## Trust model

Granting someone the admin password gives them **full shell access** to whatever
`work_dir` is set to in `system.cfg.json`, via Claude Code running with `bypassPermissions`.
Treat the admin password like a root credential:

- Use a strong random password.
- Rotate it if you suspect compromise (`make create-admin` resets it).
- Do not share it — each user who needs access should have a separate account (v2 roadmap).
- The EC2 instance itself should have restricted SSH access (key-based, no password auth).
