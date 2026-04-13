# Brainstorm: Unify System Configuration into system.cfg.json

**Date:** 2026-04-13
**Status:** Approved — ready for planning

## Problem

Changing two port numbers required editing 12 files across 5 config formats (JSON, CJS, Caddyfile, shell, Go, env templates). Config values are duplicated with no single source of truth. `system.cfg.json` exists but nothing reads it.

## Agreed Design

### Single Source of Truth: `system.cfg.json`

All non-secret, app-level configuration lives here. Secrets stay in `.env` files.

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

### Secrets (stay in `.env` files)

- `backend/.env`: `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_PASSWORD_HASH`, `CLAUDE_BIN`
- `frontend/.env.local`: `AUTH_SECRET`, `NEXTAUTH_SECRET`

### Derived Values (computed, never manually set)

| Value | Formula |
|-------|---------|
| `BACKEND_URL` | `http://localhost:${api.port}` |
| `CORS_ORIGIN` | `http://localhost:${web.port}` |
| `NEXTAUTH_URL` | `https://${hostname}` (prod) or `http://localhost:${web.port}` (dev) |
| `NEXT_PUBLIC_BACKEND_URL` | `http://localhost:${api.port}` (dev) or empty (prod) |
| Caddyfile proxy targets | `localhost:${api.port}`, `localhost:${web.port}` |
| Health check URLs | `http://localhost:${api.port}/api/health` |

### Consumer Matrix

| Consumer | Mechanism | Changes |
|----------|-----------|---------|
| `ecosystem.config.cjs` | `require('./system.cfg.json')` | Rewrite to derive all env vars from cfg |
| `start.sh` | `jq '.api.port'` etc. | Replace hardcoded URLs/ports with jq reads |
| `Caddyfile` | New `Caddyfile.template` + sed in start.sh | Template with `{{HOSTNAME}}`, `{{API_PORT}}`, `{{WEB_PORT}}` placeholders |
| `backend/internal/config/config.go` | No change | Keep hardcoded defaults as fallback for local `go run` |
| Next.js runtime | No change | Reads env vars injected by ecosystem.config.cjs |

### File Changes

**Deleted:**
- `docker-compose.yml` — no CI/CD depends on it, no Dockerfiles exist

**New:**
- `Caddyfile.template` — Caddyfile with `{{HOSTNAME}}`, `{{API_PORT}}`, `{{WEB_PORT}}` placeholders

**Rewritten:**
- `ecosystem.config.cjs` — reads system.cfg.json, computes all env vars
- `start.sh` — reads system.cfg.json via jq for all URLs/ports/defaults

**Simplified:**
- `backend/.env.example` — secrets only (PORT, CORS, worker config removed)
- `frontend/.env.local.template` — secrets only (NEXTAUTH_URL, BACKEND_URL removed)

**Updated:**
- `system.cfg.json` — expanded from 2 fields to full app config
- `README.md` — reference system.cfg.json as source of truth
- `frontend/package.json` — keep `--port 8088` for dev convenience (accepted minor drift)

**Untouched:**
- `backend/internal/config/config.go` — keep defaults as fallback
- All frontend/backend runtime code — still reads env vars

### Dependencies

- `jq` must be available on deploy target (standard on Ubuntu EC2)

## Evaluated Alternatives

### Alt 1: Runtime loading (rejected)
Go and Next.js read system.cfg.json directly at startup. Eliminates env vars for non-secret config. Rejected: adds runtime complexity, changes config loading in both apps, breaks 12-factor convention.

### Alt 2: ecosystem.config.cjs as source of truth (rejected)
Already defines ports/env vars. Rejected: can't drive Caddyfile or start.sh. CJS is awkward to parse from shell.

### Alt 3: Shared .env.shared file (rejected)
Single env file sourced by all consumers. Rejected: env files can't express nested config, Caddyfile can't source them, no type structure.

## Risk Assessment

| Risk | Mitigation |
|------|------------|
| `jq` not installed on target | start.sh checks for jq, prints install command if missing |
| Operator edits generated Caddyfile directly | Comment in generated file: "DO NOT EDIT — generated from Caddyfile.template" |
| package.json --port drifts from system.cfg.json | Rare event, dev-only impact, documented in README |
| Go defaults drift from system.cfg.json | Acceptable — PM2 always overrides. Defaults are safety net for local dev only |

## Success Criteria

- Changing any config value requires editing only `system.cfg.json`
- `./start.sh` propagates changes to all consumers
- No runtime behavior changes — Go/Next.js still read env vars
- Secrets never appear in system.cfg.json

## Next Steps

Create implementation plan with phases:
1. Expand system.cfg.json schema
2. Rewrite ecosystem.config.cjs to read from it
3. Create Caddyfile.template + generation in start.sh
4. Update start.sh to read all values via jq
5. Simplify .env templates to secrets-only
6. Delete docker-compose.yml
7. Update README.md
8. Verify: change a port in system.cfg.json, run start.sh, confirm propagation
