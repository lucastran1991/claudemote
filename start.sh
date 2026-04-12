#!/usr/bin/env bash
# claudemote deploy/boot script
# Usage: ./start.sh
# Run this on EC2 after every `git pull` to rebuild and reload processes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[claudemote]${NC} $1"; }
warn() { echo -e "${YELLOW}[claudemote]${NC} $1"; }
err()  { echo -e "${RED}[claudemote]${NC} $1" >&2; }

# ── Preflight: required binaries ─────────────────────────────────────────────
log "Checking required binaries..."
for bin in go node pnpm pm2 caddy claude; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    err "Missing required binary: $bin"
    exit 1
  fi
done
log "All binaries present."

# ── Preflight: env files ──────────────────────────────────────────────────────
if [[ ! -f backend/.env ]]; then
  err "backend/.env is missing — copy backend/.env.example and fill in secrets."
  exit 1
fi

if [[ ! -f frontend/.env.local ]]; then
  err "frontend/.env.local is missing — copy frontend/.env.local.template and fill in values."
  exit 1
fi

# ── DB directory ─────────────────────────────────────────────────────────────
log "Ensuring DB directory exists..."
sudo mkdir -p /var/lib/claudemote
sudo chown "$USER" /var/lib/claudemote

# ── Log directory ─────────────────────────────────────────────────────────────
mkdir -p logs

# ── Build: backend ────────────────────────────────────────────────────────────
log "Building Go backend..."
( cd backend && go build -o server ./cmd/server )
log "Backend built."

# ── Build: frontend ───────────────────────────────────────────────────────────
log "Installing frontend dependencies..."
( cd frontend && pnpm install --frozen-lockfile )
log "Building frontend..."
( cd frontend && pnpm build )
log "Frontend built."

# ── pm2 ───────────────────────────────────────────────────────────────────────
log "Starting/reloading pm2 processes..."
pm2 startOrReload ecosystem.config.cjs
pm2 save

echo ""
log "claudemote is running:"
pm2 ls
echo ""
log "API  → http://localhost:8080/api/health"
log "Web  → http://localhost:3000"
log "Logs → pm2 logs  (or: make logs)"
