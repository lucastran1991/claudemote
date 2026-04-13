# Deployment Guide

## Prerequisites

| Tool | Min Version | Install |
|------|-------------|---------|
| Go | 1.22 | `sudo apt install golang-go` or [go.dev](https://go.dev/dl/) |
| Node.js | 20 | [nodesource](https://github.com/nodesource/distributions) |
| pnpm | latest | `npm install -g pnpm` (or `corepack enable && corepack prepare pnpm@latest`) |
| pm2 | latest | `npm install -g pm2` |
| Caddy | 2.8.4+ | Auto-installed by `./start.sh --bootstrap` or `sudo apt install caddy` |
| Claude Code CLI | latest | [docs.anthropic.com](https://docs.anthropic.com/claude-code) — must be authenticated |
| jq | latest | `sudo apt install jq` (required by `./start.sh` for config parsing) |

EC2 security group: Open inbound TCP **80** and **443** (HTTP/HTTPS).

## First Deploy (./start.sh --bootstrap)

Three-phase bootstrapping. Run once per fresh EC2.

### Phase 1: Discovery
Detects OS, installs missing tools, verifies Claude auth, prompts for config.

```bash
./start.sh --bootstrap
```

You'll be asked for:
1. **WORK_DIR** — Git repo Claude Code should operate on (default: repo root)
2. **Caddy hostname** — Public domain (e.g. `claudemote.example.com` or `<IP>.nip.io`)
3. **Public URLs** — NEXTAUTH_URL and BACKEND_URL for the application
4. **Admin credentials** — Username/password for web login

**DNS setup (before Phase 2):**
- Create A record pointing your hostname to the EC2 public IP
- Caddy needs this for Let's Encrypt TLS provisioning
- If using `*.nip.io`, DNS is automatic (no action needed)

### Phase 2: Configuration
Writes env files and generates Caddyfile from template.

Generated files:
- `backend/.env` — JWT_SECRET, admin credentials, paths
- `frontend/.env.local` — AUTH_SECRET, NEXTAUTH_URL
- `/etc/caddy/Caddyfile` — reverse proxy config (generated from `Caddyfile.template`)

### Phase 3: Build & Start
Compiles code, starts services, verifies runtime.

Checks:
- Go backend responds on `/api/health`
- Next.js responds on configured web port
- HTTPS cert issued (may take 30-60s on first run)

### Remaining Manual Steps

After bootstrap completes:

```bash
# 1. EC2 security group — inbound TCP 80, 443 to 0.0.0.0/0

# 2. pm2 startup persistence (survives EC2 reboot)
pm2 startup systemd -u $USER --hp $HOME
# Then run the printed sudo command, then:
pm2 save

# 3. (Optional) Change default admin password
# Log in to web UI as admin/Password@123, change password
```

## Recurring Deploys (After Code Changes)

```bash
git pull
./start.sh          # build backend + frontend, reload pm2
```

Or, reload without rebuilding:

```bash
make reload
```

## Configuration Changes

**Non-secret config** (ports, hostname, worker settings, paths) lives in `system.cfg.json`:

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

To change any value:
1. Edit `system.cfg.json`
2. Run `./start.sh`
3. Changes are picked up by pm2 reload

**Secret config** lives in `.env` files:
- `backend/.env` — JWT_SECRET, admin password hash, CLAUDE_BIN path
- `frontend/.env.local` — NextAuth secrets

To change secrets:
1. Edit `.env` file
2. Run `./start.sh` (rebuilds and reloads) OR `pm2 reload ecosystem.config.cjs`

## Troubleshooting

### start.sh fails at Phase 1
Check the error message — usually missing tool or Claude not authenticated.

```bash
# Claude auth check (two valid states):
# 1. ANTHROPIC_API_KEY in shell env
export ANTHROPIC_API_KEY=sk-ant-...

# 2. Or, run OAuth flow
claude
# Then try ./start.sh --bootstrap again
```

### start.sh fails at Phase 2 (Caddyfile generation)
Caddy validate error — check the error output. Common issues:
- Hostname not resolvable (DNS not propagated yet)
- Caddyfile template syntax error (unlikely, report as bug)

Workaround: Wait for DNS to propagate, then retry.

### start.sh fails at Phase 3 (runtime verification)
Backend or frontend failing to start.

```bash
pm2 logs claudemote-api     # see backend startup errors
pm2 logs claudemote-web     # see Next.js startup errors
pm2 status                  # process status
```

### HTTPS cert not issued yet
Let's Encrypt can take 30-60s on first deploy.

```bash
# Check Caddy status
sudo systemctl status caddy
sudo journalctl -u caddy -n 50  # recent logs

# Retry after a minute
curl -v https://<hostname>/api/health
```

### Can't reach backend/frontend locally
Check ports in `system.cfg.json` and ensure no other process is listening.

```bash
# Check if ports are in use
lsof -i :8888  # API port
lsof -i :8088  # Web port

# Or use pm2
pm2 ls
```

### Jobs not running / workers seem stuck
Check worker count and job timeout in `system.cfg.json`.

```bash
# Check job queue length and worker pool
curl -H "Authorization: Bearer <token>" http://localhost:8888/api/debug/stats

# Check backend logs
pm2 logs claudemote-api | tail -100
```

### Claude binary not found during bootstrap
`start.sh --bootstrap` detects `claude` in PATH.

```bash
# Verify Claude is installed and in PATH
which claude

# If not found, install from docs.anthropic.com
# Then retry ./start.sh --bootstrap
```

## Make Targets

Common deployment commands:

| Target | Description |
|--------|-------------|
| `make dev` | Start servers locally without pm2 (useful for local testing) |
| `make build` | Compile Go binary + Next.js production build |
| `make test` | Run backend Go tests + frontend tests |
| `make migrate-up` | Apply DB migrations |
| `make create-admin` | Create/reset admin user interactively |
| `make deploy` | `build` + `reload` (full deploy cycle) |
| `make reload` | `pm2 reload ecosystem.config.cjs` (restart without rebuild) |
| `make logs` | Tail all pm2 logs |
| `make clean` | Remove `backend/server` and `frontend/.next` |

## Monitoring

### Process Status
```bash
pm2 ls                  # process list + uptime + memory
pm2 logs claudemote-api # stream backend logs
pm2 logs claudemote-web # stream frontend logs
pm2 logs                # stream all
```

### Caddy Status
```bash
sudo systemctl status caddy
sudo journalctl -u caddy -n 50
```

### Database
```bash
# Check job count (requires sqlite3)
sqlite3 /var/lib/claudemote/claudemote.db "SELECT COUNT(*) FROM jobs;"
```

### Health Checks
```bash
# Backend health
curl -s http://localhost:8888/api/health | jq .

# Frontend
curl -sI http://localhost:8088

# HTTPS (once cert is issued)
curl -s https://<hostname>/api/health | jq .
```

## Advanced: Custom Work Directory

To operate on a different repo (instead of the claudemote repo itself), edit `system.cfg.json:work_dir`:

```json
{
  "work_dir": "/path/to/your/repo"
}
```

Then deploy:
```bash
./start.sh
```

The Go backend will chdir to `work_dir` before running Claude Code subprocesses.

## Advanced: Force Bootstrap

To reset env files and re-prompt for everything:

```bash
./start.sh --bootstrap --force
```

This wipes `backend/.env` and `frontend/.env.local` and re-runs Phase 1 discovery. Useful if secrets have changed or you need to reconfigure.

## Advanced: Smoke Test

Bootstrap with end-to-end job validation:

```bash
./start.sh --bootstrap --smoke-test
```

After Phase 3, submits a trivial Claude Code job and polls for completion. Verifies worker pool, job queue, and SSE streaming are working.
