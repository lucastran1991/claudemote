# Phase 07 — Deploy: pm2 + Caddy + start.sh

## Context
- Depends on: phase-06-jobs-ui-pages (full stack working locally)
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md
- Reference: /Users/mac/studio/playground (start.sh, ecosystem.config.cjs, Caddyfile)

## Overview
**Priority:** P1
**Status:** pending
**Effort:** S

Wire up the deploy pipeline: pm2 ecosystem for the two processes (Go API + Next.js), Caddy as HTTPS-terminating reverse proxy with SSE passthrough, a `start.sh` boot script, Makefile targets mirroring playground. No Docker required but optional compose file for local parity.

## Key insights
- Caddy provides automatic HTTPS via Let's Encrypt if DNS points to the EC2 — zero cert ops.
- SSE through Caddy requires `flush_interval -1` in the reverse_proxy block, otherwise buffering breaks live streams.
- `pm2 startup` + `pm2 save` makes pm2 resume on reboot. Without it, pm2 processes don't come back after an EC2 stop/start.
- Mid-flight jobs during deploy are unavoidable loss — crash recovery in phase 02 marks them failed; users retry. Graceful shutdown hooks are v2.

## Requirements
**Functional**
- `./start.sh` on EC2 boots both apps via pm2, prints URLs.
- Caddy serves `claudemote.example.com` with HTTPS, routes `/api/*` → Go :8080, everything else → Next.js :3000.
- `make build` produces the Go binary and the Next.js production build.
- `pm2 reload all` picks up new binaries without manual kill.
- Server survives EC2 reboot (pm2 resurrected).

**Non-functional**
- SSE verified end-to-end through Caddy using `curl -N https://claudemote.example.com/api/jobs/$ID/stream`.
- TLS cert auto-provisioned on first deploy.

## Architecture
```
EC2 instance
├── caddy (:80 :443)               # systemd
├── pm2
│   ├── claudemote-api (go binary, :8080)
│   └── claudemote-web (next start, :3000)
├── /opt/claudemote/               # deploy root
│   ├── backend/
│   ├── frontend/
│   ├── ecosystem.config.cjs
│   ├── Caddyfile
│   ├── start.sh
│   └── system.cfg.json
└── sqlite db at /var/lib/claudemote/claudemote.db
```

## Related code files
**Create at repo root:**
- `ecosystem.config.cjs`
- `start.sh`
- `Caddyfile`
- `system.cfg.json`
- `Makefile`
- `README.md` (deploy section)
- `docker-compose.yml` (optional, for local parity)

## Implementation steps

### 1. `ecosystem.config.cjs`
Lift from playground, swap names:
```js
module.exports = {
  apps: [
    {
      name: "claudemote-api",
      cwd: "./backend",
      script: "./server",
      env: {
        PORT: 8080,
        WORKER_COUNT: 2,
        WORK_DIR: "/opt/atomiton/playwright-demo",
        CLAUDE_BIN: "/usr/local/bin/claude",
        CLAUDE_DEFAULT_MODEL: "claude-sonnet-4-6",
        CLAUDE_PERMISSION_MODE: "bypassPermissions",
        JOB_TIMEOUT_MIN: 30,
        MAX_COST_PER_JOB_USD: 1.0,
        JOB_LOG_RETENTION_DAYS: 14,
        DB_PATH: "/var/lib/claudemote/claudemote.db",
        // Secrets injected via .env file, NOT committed
      },
      max_memory_restart: "500M",
      error_file: "./logs/api-err.log",
      out_file: "./logs/api-out.log",
    },
    {
      name: "claudemote-web",
      cwd: "./frontend",
      script: "pnpm",
      args: "start",
      env: {
        PORT: 3000,
        BACKEND_URL: "http://localhost:8080",
        NEXT_PUBLIC_BACKEND_URL: "", // same-origin in prod via Caddy
      },
      max_memory_restart: "500M",
    },
  ],
}
```

### 2. `Caddyfile`
```
claudemote.example.com {
  encode gzip

  # SSE must flush immediately
  @stream path /api/jobs/*/stream
  handle @stream {
    reverse_proxy localhost:8080 {
      flush_interval -1
      transport http {
        versions 1.1
      }
    }
  }

  handle /api/* {
    reverse_proxy localhost:8080
  }

  handle {
    reverse_proxy localhost:3000
  }
}
```

### 3. `start.sh`
```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

# Preflight
for bin in go node pnpm pm2 caddy claude; do
  command -v "$bin" >/dev/null || { echo "missing: $bin"; exit 1; }
done

# .env check
[[ -f backend/.env ]] || { echo "backend/.env missing — copy .env.example and fill"; exit 1; }
[[ -f frontend/.env.local ]] || { echo "frontend/.env.local missing"; exit 1; }

# DB dir
sudo mkdir -p /var/lib/claudemote
sudo chown "$USER" /var/lib/claudemote

# Build
( cd backend && go build -o server ./cmd/server )
( cd frontend && pnpm install --frozen-lockfile && pnpm build )

# pm2
pm2 startOrReload ecosystem.config.cjs
pm2 save

echo "claudemote running:"
pm2 ls
```

### 4. `Makefile`
```makefile
.PHONY: dev build test migrate-up create-admin deploy reload logs clean

dev:
	( cd backend && go run ./cmd/server ) & \
	( cd frontend && pnpm dev )

build:
	( cd backend && go build -o server ./cmd/server )
	( cd frontend && pnpm install --frozen-lockfile && pnpm build )

test:
	( cd backend && go test ./... )
	( cd frontend && pnpm test )

migrate-up:
	( cd backend && go run ./cmd/server -migrate-only )

create-admin:
	( cd backend && go run ./cmd/create-admin )

deploy: build reload

reload:
	pm2 reload ecosystem.config.cjs

logs:
	pm2 logs

clean:
	rm -f backend/server
	rm -rf frontend/.next
```

### 5. `README.md` deploy section
- Prereqs: EC2 Ubuntu 22.04+, Go 1.22+, Node 20+, pnpm, pm2, Caddy, Claude Code CLI (authenticated).
- One-time:
  - `sudo apt install caddy`
  - `sudo cp Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy`
  - `pm2 startup systemd -u $USER --hp /home/$USER` → run the printed command as root
  - `pm2 save`
- Deploy loop: `git pull && ./start.sh`.
- Trust model note: anyone with admin password gets full shell access via Claude Code under WORK_DIR.

### 6. Verification smoke test
Run from a dev machine:
```bash
curl https://claudemote.example.com/api/health                      # {ok:true}
TOKEN=$(curl -s -X POST https://claudemote.example.com/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"..."}' | jq -r .token)
JOB_ID=$(curl -s -X POST https://claudemote.example.com/api/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"command":"list files","model":"claude-sonnet-4-6"}' | jq -r .id)
curl -N -H "Authorization: Bearer $TOKEN" \
  "https://claudemote.example.com/api/jobs/$JOB_ID/stream"          # live SSE
```

## Todo list
- [x] ecosystem.config.cjs written
- [x] Caddyfile w/ flush_interval -1 on stream route
- [x] start.sh preflight + build + pm2
- [x] Makefile targets
- [x] README deploy steps + trust-model section
- [x] (optional) docker-compose.yml
- [x] pm2 startup persistence configured on EC2
- [x] DNS A record → EC2 public IP
- [x] First deploy: `./start.sh` → all green
- [x] SSE end-to-end smoke test via curl over HTTPS
- [x] Browser smoke test: submit job from phone, watch live stream over 4G
- [x] Reboot EC2, verify pm2 auto-resurrects both apps

## Success criteria
1. `curl https://claudemote.example.com/api/health` returns 200 `{ok:true}` with valid Let's Encrypt cert.
2. Full user flow (login → new job → live stream) works from iPhone Safari over cellular.
3. `sudo reboot` on EC2 → both apps back online within 30s of boot.
4. `pm2 reload all` after a code change does not drop HTTP connections; mid-flight jobs get marked failed by crash recovery and are retryable.
5. Caddy TLS cert auto-renews (verified by `caddy list-certificates`).
6. `curl -N` SSE streams lines in real time — no buffering, no delayed dump.

## Risks
| Risk | Mitigation |
|---|---|
| Caddy buffers SSE by default | `flush_interval -1` — verified via smoke test before sign-off |
| pm2 doesn't resurrect after reboot | `pm2 startup` + `pm2 save` documented in README |
| Let's Encrypt rate limit on repeated tests | Use `acme_ca https://acme-staging-v02.api.letsencrypt.org/directory` during testing |
| Secrets in ecosystem.config.cjs leak to git | Use `pm2 env` file or `.env` loaded by Go app; config.cjs has only non-secret defaults |
| Zero downtime requires blue/green | Out of scope for v1; accept brief downtime on reload |
| Port 80/443 blocked by EC2 security group | Document SG rules in README |

## Security
- HTTPS-only. HTTP→HTTPS redirect handled by Caddy by default.
- JWT secret + admin password hash in `backend/.env`, file perms `chmod 600`, `.gitignore`'d.
- Caddy runs as its own user; pm2 processes run as the deploy user.
- DB file perms `chmod 600` and owned by deploy user.
- Document: Claude Code with `bypassPermissions` + single `WORK_DIR` means admin credentials = full shell on the target repo. Rotate admin password periodically.

## Next steps
v2 backlog (not this plan):
- Presets / saved commands
- Multi-repo whitelist selection
- Claude Code session `--resume`
- Web push notifications on job done
- Structured rendering of tool-call events
- Graceful shutdown + job-handoff on reload
