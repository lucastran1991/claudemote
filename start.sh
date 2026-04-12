#!/usr/bin/env bash
# claudemote deploy/boot script
# Usage:
#   ./start.sh               — build + reload pm2 (default)
#   ./start.sh --bootstrap   — first-run setup (Caddy install, env files, admin,
#                              hostname) then deploy
#   ./start.sh --help        — show this help
#
# Run --bootstrap once per fresh EC2; thereafter run plain ./start.sh after
# every `git pull`.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${GREEN}[claudemote]${NC} $1"; }
info() { echo -e "${BLUE}[claudemote]${NC} $1"; }
warn() { echo -e "${YELLOW}[claudemote]${NC} $1"; }
err()  { echo -e "${RED}[claudemote]${NC} $1" >&2; }

usage() {
  cat <<EOF
Usage: ./start.sh [--bootstrap|--help]

  (no flag)     Build + reload pm2 processes (recurring deploy).
  --bootstrap   First-run setup: install Caddy if missing, generate secrets,
                prompt for hostname/admin, build backend, create admin user,
                then deploy. Run once per fresh EC2.
  --help        Show this message.
EOF
}

# ── Helpers ──────────────────────────────────────────────────────────────────

require_bin() {
  local missing=()
  for bin in "$@"; do
    command -v "$bin" >/dev/null 2>&1 || missing+=("$bin")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    err "Missing required binaries: ${missing[*]}"
    exit 1
  fi
}

prompt() {
  # prompt "Question" [default] → echoes user input (or default) on stdout
  local q="$1" def="${2:-}" answer
  if [[ -n "$def" ]]; then
    read -r -p "$q [$def]: " answer </dev/tty
    echo "${answer:-$def}"
  else
    read -r -p "$q: " answer </dev/tty
    echo "$answer"
  fi
}

prompt_secret() {
  # prompt_secret "Question" → reads twice with confirm, echoes value on stdout
  local q="$1" pw1 pw2
  while true; do
    read -r -s -p "$q: " pw1 </dev/tty; echo >&2
    read -r -s -p "Confirm: "  pw2 </dev/tty; echo >&2
    if [[ -n "$pw1" && "$pw1" == "$pw2" ]]; then
      echo "$pw1"; return 0
    fi
    warn "Passwords did not match (or empty). Try again."
  done
}

# ── Preflight ────────────────────────────────────────────────────────────────

check_binaries() {
  log "Checking required binaries..."
  require_bin go node pnpm pm2 caddy claude
  log "All binaries present."
}

check_envs() {
  if [[ ! -f backend/.env ]]; then
    err "backend/.env is missing — run: ./start.sh bootstrap"
    exit 1
  fi
  if [[ ! -f frontend/.env.local ]]; then
    err "frontend/.env.local is missing — run: ./start.sh bootstrap"
    exit 1
  fi
}

# ── Bootstrap (one-time, interactive) ────────────────────────────────────────

install_pnpm_if_missing() {
  if command -v pnpm >/dev/null 2>&1; then
    info "pnpm already installed: $(pnpm --version 2>&1)"
    return 0
  fi
  log "pnpm not found — installing via corepack..."

  # Strategy 1: corepack (ships with Node ≥16.10, the modern path)
  if command -v corepack >/dev/null 2>&1; then
    if sudo corepack enable >/dev/null 2>&1 \
       && corepack prepare pnpm@latest --activate >/dev/null 2>&1; then
      log "pnpm installed via corepack: $(pnpm --version 2>&1)"
      return 0
    fi
    warn "corepack install failed — falling back to npm global install."
  else
    warn "corepack not present — falling back to npm global install."
  fi

  # Strategy 2: npm global install
  if command -v npm >/dev/null 2>&1; then
    if sudo npm install -g pnpm >/dev/null 2>&1; then
      log "pnpm installed via npm: $(pnpm --version 2>&1)"
      return 0
    fi
  fi

  err "Could not install pnpm automatically. Install manually:"
  err "  sudo corepack enable && corepack prepare pnpm@latest --activate"
  err "  OR: sudo npm install -g pnpm"
  exit 1
}

install_caddy_if_missing() {
  if command -v caddy >/dev/null 2>&1; then
    info "Caddy already installed: $(caddy version 2>&1 | head -1)"
    return 0
  fi
  log "Caddy not found — installing..."

  # Strategy 1: yum/dnf + Fedora COPR (works on AL2023 cleanly, often on AL2)
  local pm=""
  if command -v dnf >/dev/null 2>&1; then
    pm=dnf
  elif command -v yum >/dev/null 2>&1; then
    pm=yum
  fi

  if [[ -n "$pm" ]]; then
    info "Trying ${pm} + COPR @caddy/caddy..."
    if sudo "$pm" install -y "${pm}-plugins-core" >/dev/null 2>&1 \
       && sudo "$pm" copr enable -y @caddy/caddy >/dev/null 2>&1 \
       && sudo "$pm" install -y caddy >/dev/null 2>&1; then
      log "Caddy installed via ${pm}/COPR: $(caddy version 2>&1 | head -1)"
      return 0
    fi
    warn "${pm}/COPR install failed — falling back to GitHub binary."
  else
    warn "No yum/dnf found — falling back to GitHub binary."
  fi

  # Strategy 2: download static binary from GitHub releases
  install_caddy_from_github
}

install_caddy_from_github() {
  require_bin curl tar
  local arch version url tmp
  arch="$(uname -m)"
  case "$arch" in
    x86_64)  arch=amd64 ;;
    aarch64) arch=arm64 ;;
    arm64)   arch=arm64 ;;
    *) err "Unsupported arch for Caddy install: $arch"; exit 1 ;;
  esac
  version="2.8.4"
  url="https://github.com/caddyserver/caddy/releases/download/v${version}/caddy_${version}_linux_${arch}.tar.gz"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  log "Downloading caddy v${version} for linux/${arch}..."
  curl -fsSL "$url" -o "$tmp/caddy.tar.gz"
  tar -xzf "$tmp/caddy.tar.gz" -C "$tmp" caddy
  sudo install -m 755 "$tmp/caddy" /usr/local/bin/caddy
  log "Caddy installed: $(caddy version 2>&1 | head -1)"
}

bootstrap_backend_env() {
  if [[ -f backend/.env ]]; then
    warn "backend/.env already exists — keeping it."
    return 0
  fi
  [[ -f backend/.env.example ]] || { err "backend/.env.example not found"; exit 1; }

  log "Generating backend/.env..."
  cp backend/.env.example backend/.env
  local jwt workdir
  jwt="$(openssl rand -hex 32)"

  # WORK_DIR is the codebase Claude jobs will read/write/run commands in.
  # Default = self-hosting mode: claudemote operates on its own source.
  # Press Enter to accept the default, or supply an absolute path to a
  # different repo on this host.
  echo
  info "WORK_DIR is the repo Claude jobs will read/write/run commands in."
  info "  Default = $ROOT (self-hosting: claudemote operates on its own source)."
  info "  Override only if you want Claude to operate on a different repo."
  echo
  workdir="$(prompt 'WORK_DIR' "$ROOT")"
  if [[ -z "$workdir" || ! -d "$workdir" ]]; then
    err "WORK_DIR must be an existing absolute path."
    exit 1
  fi
  if [[ "$workdir" == "$ROOT" ]]; then
    info "Self-hosting mode: claudemote will operate on its own source tree."
  fi
  # Portable in-place sed (creates .bak on both macOS and GNU; cleaned after)
  sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=${jwt}|" backend/.env
  sed -i.bak "s|^WORK_DIR=.*|WORK_DIR=${workdir}|" backend/.env
  rm -f backend/.env.bak
  log "backend/.env written (JWT_SECRET generated, WORK_DIR=${workdir})."
}

bootstrap_frontend_env() {
  if [[ -f frontend/.env.local ]]; then
    warn "frontend/.env.local already exists — keeping it."
    return 0
  fi
  [[ -f frontend/.env.local.template ]] || { err "frontend/.env.local.template not found"; exit 1; }

  log "Generating frontend/.env.local..."
  cp frontend/.env.local.template frontend/.env.local
  local secret nextauth_url backend_url
  secret="$(openssl rand -base64 32)"
  nextauth_url="$(prompt 'Public URL of the Next.js app (NEXTAUTH_URL)' 'http://localhost:3000')"
  backend_url="$(prompt 'Backend URL for server-side fetch (BACKEND_URL)' 'http://localhost:8080')"
  sed -i.bak "s|^AUTH_SECRET=.*|AUTH_SECRET=${secret}|" frontend/.env.local
  sed -i.bak "s|^NEXTAUTH_SECRET=.*|NEXTAUTH_SECRET=${secret}|" frontend/.env.local
  sed -i.bak "s|^NEXTAUTH_URL=.*|NEXTAUTH_URL=${nextauth_url}|" frontend/.env.local
  sed -i.bak "s|^BACKEND_URL=.*|BACKEND_URL=${backend_url}|" frontend/.env.local
  sed -i.bak 's|^NEXT_PUBLIC_BACKEND_URL=.*|NEXT_PUBLIC_BACKEND_URL=|' frontend/.env.local
  rm -f frontend/.env.local.bak
  log "frontend/.env.local written (AUTH_SECRET generated)."
}

bootstrap_caddy_hostname() {
  if ! grep -q "claudemote.example.com" Caddyfile 2>/dev/null; then
    info "Caddyfile hostname already customized — skipping."
    return 0
  fi
  local hostname
  hostname="$(prompt 'Public hostname for Caddy (e.g. claudemote.foo.com — blank to skip)' '')"
  if [[ -z "$hostname" ]]; then
    warn "Caddyfile hostname unchanged. Edit before exposing publicly."
    return 0
  fi
  sed -i.bak "s|claudemote.example.com|${hostname}|g" Caddyfile
  rm -f Caddyfile.bak
  log "Caddyfile hostname set to ${hostname}."
  warn "Run: sudo cp Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy"
}

bootstrap_admin_user() {
  log "Creating admin user (one-time)..."
  local user pass
  user="$(prompt 'Admin username' 'admin')"
  pass="$(prompt_secret 'Admin password (min 8 chars)')"
  if [[ ${#pass} -lt 8 ]]; then
    err "Password too short (min 8)."
    exit 1
  fi
  ( cd backend && ADMIN_USERNAME="$user" ADMIN_PASSWORD="$pass" go run ./cmd/create-admin )
  log "Admin user '${user}' ready."
}

print_remaining_manual_steps() {
  cat <<'EOF'

[claudemote] Bootstrap done. Steps that still need manual action:

  1. claude CLI authentication — run as the same user that owns this script:
       claude          # follow OAuth flow
       # OR export ANTHROPIC_API_KEY=sk-ant-... in this user's shell rc

  2. pm2 startup persistence (survives reboot):
       pm2 startup systemd -u $USER --hp $HOME
       # then run the printed sudo command, then:
       pm2 save

  3. DNS — point an A record at this EC2 public IP.

  4. EC2 security group — open inbound TCP 80 and 443.

  5. Caddy site config — Caddy is installed, but the site file isn't deployed:
       sudo cp Caddyfile /etc/caddy/Caddyfile
       sudo systemctl enable --now caddy
       sudo systemctl reload caddy

EOF
}

bootstrap() {
  log "First-run bootstrap starting..."
  # Hard prereqs the user MUST install themselves (no auto-install for these)
  require_bin go node pm2 claude openssl
  # pnpm: try corepack/npm install if missing
  install_pnpm_if_missing
  # Caddy: try yum/dnf+COPR or GitHub binary if missing
  install_caddy_if_missing
  # Now everything required for deploy must be present
  require_bin pnpm caddy

  bootstrap_backend_env
  bootstrap_frontend_env
  bootstrap_caddy_hostname
  log "Building backend (needed for create-admin)..."
  ( cd backend && go build -o server ./cmd/server )
  bootstrap_admin_user
  print_remaining_manual_steps
  log "Continuing with deploy..."
}

# ── Build & deploy (recurring) ───────────────────────────────────────────────

ensure_dirs() {
  log "Ensuring DB and log directories..."
  sudo mkdir -p /var/lib/claudemote
  sudo chown "$USER" /var/lib/claudemote
  mkdir -p logs
}

build_backend() {
  log "Building Go backend..."
  ( cd backend && go build -o server ./cmd/server )
}

build_frontend() {
  log "Installing frontend dependencies..."
  ( cd frontend && pnpm install --frozen-lockfile )
  log "Building frontend..."
  ( cd frontend && pnpm build )
}

pm2_reload() {
  log "Starting/reloading pm2 processes..."
  pm2 startOrReload ecosystem.config.cjs
  pm2 save
}

print_endpoints() {
  echo ""
  log "claudemote is running:"
  pm2 ls
  echo ""
  log "API  → http://localhost:8080/api/health"
  log "Web  → http://localhost:3000"
  log "Logs → pm2 logs  (or: make logs)"
}

deploy() {
  check_binaries
  check_envs
  ensure_dirs
  build_backend
  build_frontend
  pm2_reload
  print_endpoints
}

# ── Dispatch ─────────────────────────────────────────────────────────────────

case "${1:-}" in
  --bootstrap)
    # bootstrap() handles its own preflight (incl. Caddy install)
    bootstrap
    check_envs
    ensure_dirs
    # backend already built by bootstrap; reuse the binary
    build_frontend
    pm2_reload
    print_endpoints
    ;;
  "")
    deploy
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    err "Unknown arg: $1"
    usage
    exit 1
    ;;
esac
