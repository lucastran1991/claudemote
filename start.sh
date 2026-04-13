#!/usr/bin/env bash
# claudemote deploy/boot script
#
# Usage:
#   ./start.sh                              Recurring deploy (build + reload pm2).
#   ./start.sh --bootstrap                  First-run 3-phase setup.
#   ./start.sh --bootstrap --force          Wipe .env files, then bootstrap.
#   ./start.sh --bootstrap --smoke-test     Bootstrap + end-to-end job test.
#   ./start.sh --help                       Show this help.
#
# Run --bootstrap once per fresh EC2; thereafter run plain ./start.sh after
# every `git pull`.
#
# Bootstrap is split into 3 hard-gated phases:
#   Phase 1 DISCOVER    — detect/install tools, verify, collect all inputs.
#   Phase 2 CONFIGURE   — write env files + caddy config. No builds.
#   Phase 3 BUILD&START — compile, migrate, start services, verify runtime.
#
# Exit codes: 0=ok, 1=bad-invocation, 10=phase-1-fail, 20=phase-2-fail,
#             30=phase-3-fail.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_DIR="$ROOT"
cd "$ROOT"

# ── system.cfg.json — single source of truth for all non-secret config ───────
CFG_FILE="${SCRIPT_DIR}/system.cfg.json"
cfg() { jq -r "$1" "$CFG_FILE"; }

# Fail fast if jq is missing — everything below depends on it.
command -v jq >/dev/null 2>&1 || {
  echo "ERROR: jq is required but not installed. Install with: sudo apt install jq"
  exit 1
}

[[ -f "$CFG_FILE" ]] || { echo "ERROR: $CFG_FILE not found"; exit 1; }
jq empty "$CFG_FILE" 2>/dev/null || { echo "ERROR: $CFG_FILE is not valid JSON"; exit 1; }

API_PORT="$(cfg '.api.port')"
WEB_PORT="$(cfg '.web.port')"
CFG_HOSTNAME="$(cfg '.hostname')"

# ── Constants ────────────────────────────────────────────────────────────────
readonly BS_PROD_DB_PATH="/var/lib/claudemote/claudemote.db"
readonly CADDY_VERSION="2.8.4"

# ── Flags (set by dispatch parser) ───────────────────────────────────────────
BS_FORCE=0
BS_SMOKE_TEST=0

# ── Script globals populated during Phase 1, consumed by Phase 2+3 ───────────
BS_OS=""
BS_CLAUDE_BIN=""
BS_PUBLIC_IP=""
BS_WORK_DIR=""
BS_HOSTNAME=""
BS_NEXTAUTH_URL=""
BS_BACKEND_URL=""
BS_ADMIN_USER="admin"
BS_ADMIN_PASS="Password@123"
BS_JWT_SECRET=""
BS_AUTH_SECRET=""

# ── Colors + log helpers ─────────────────────────────────────────────────────
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
Usage: ./start.sh [--bootstrap] [--force] [--smoke-test] [--help]

  (no flag)     Build + reload pm2 processes (recurring deploy).
  --bootstrap   First-run 3-phase setup: detect tools, prompt for config,
                write env files, build, create admin, start services.
                Run once per fresh EC2.
  --force       With --bootstrap: wipe backend/.env and frontend/.env.local
                first so Phase 1 re-prompts for everything. Destructive.
  --smoke-test  With --bootstrap: after Phase 3, submit one trivial job via
                the API and verify end-to-end (worker pool, claude, SSE).
                Soft — warns on failure, does not exit nonzero.
  --help        Show this message.

Exit codes: 0 success, 1 bad-invocation, 10 discovery-fail,
            20 configure-fail, 30 build/start-fail.
EOF
}

# ── Primitives ───────────────────────────────────────────────────────────────

require_bin() {
  local missing=()
  for bin in "$@"; do
    command -v "$bin" >/dev/null 2>&1 || missing+=("$bin")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    err "Missing required binaries: ${missing[*]}"
    exit 10
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

checkpoint() {
  local phase="$1"
  echo
  log "✓ ${phase} complete"
  echo
}

fail_phase() {
  local phase="$1" step="$2" reason="$3" code="$4"
  echo
  err "✗ ${phase} failed at: ${step}"
  err "  reason: ${reason}"
  err "  recover: fix the issue above, then re-run ./start.sh --bootstrap"
  exit "$code"
}

# ── Recurring deploy helpers (used by both bootstrap and plain deploy) ───────

check_binaries() {
  log "Checking required binaries..."
  require_bin go node pnpm pm2 caddy claude
  log "All binaries present."
}

check_envs() {
  if [[ ! -f backend/.env ]]; then
    err "backend/.env is missing — run: ./start.sh --bootstrap"
    exit 1
  fi
  if [[ ! -f frontend/.env.local ]]; then
    err "frontend/.env.local is missing — run: ./start.sh --bootstrap"
    exit 1
  fi
}

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
  log "API  → http://localhost:${API_PORT}/api/health"
  log "Web  → http://localhost:${WEB_PORT}"
  log "Logs → pm2 logs  (or: make logs)"
  if [[ -n "$BS_HOSTNAME" ]]; then
    log "Public → https://${BS_HOSTNAME}"
  fi
  if [[ -n "$BS_ADMIN_USER" ]]; then
    log "Admin → ${BS_ADMIN_USER} / ${BS_ADMIN_PASS}"
    if [[ "$BS_ADMIN_PASS" == "Password@123" ]]; then
      warn "USING DEFAULT ADMIN CREDENTIALS — change password after first login!"
    fi
  fi
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

# ═════════════════════════════════════════════════════════════════════════════
#   PHASE 1 — DISCOVERY (read-only, idempotent)
# ═════════════════════════════════════════════════════════════════════════════

bs_detect_os() {
  if [[ -f /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    case "${ID:-}" in
      amzn)   BS_OS="al${VERSION_ID:-2023}" ;;
      ubuntu) BS_OS="ubuntu" ;;
      debian) BS_OS="debian" ;;
      fedora) BS_OS="fedora" ;;
      *)      BS_OS="linux" ;;
    esac
  elif [[ "$(uname -s)" == "Darwin" ]]; then
    BS_OS="macos-dev"
  else
    BS_OS="unknown"
  fi
  info "OS family: $BS_OS"
}

bs_install_pnpm_if_missing() {
  if command -v pnpm >/dev/null 2>&1; then
    info "pnpm already installed: $(pnpm --version 2>&1)"
    return 0
  fi
  log "pnpm not found — installing via corepack..."

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

  if command -v npm >/dev/null 2>&1 && sudo npm install -g pnpm >/dev/null 2>&1; then
    log "pnpm installed via npm: $(pnpm --version 2>&1)"
    return 0
  fi

  fail_phase "Phase 1/3 Discovery" "install pnpm" \
    "corepack and npm global install both failed" 10
}

# Creates the full caddy systemd service stack: user, group, /etc/caddy,
# unit file. Idempotent — safe to re-run. Called after the GitHub-binary
# install path since dnf/COPR sets all this up itself.
install_caddy_systemd_bundle() {
  log "Installing caddy systemd service stack..."

  if ! getent group caddy >/dev/null 2>&1; then
    sudo groupadd --system caddy
  fi
  if ! id caddy >/dev/null 2>&1; then
    sudo useradd --system --gid caddy \
      --create-home --home-dir /var/lib/caddy \
      --shell /usr/sbin/nologin \
      --comment "Caddy web server" caddy
  fi

  sudo mkdir -p /etc/caddy
  # Placeholder so caddy can start before Phase 2 writes the real site config.
  if [[ ! -f /etc/caddy/Caddyfile ]]; then
    echo ':80 { respond "claudemote bootstrap placeholder" 200 }' \
      | sudo tee /etc/caddy/Caddyfile >/dev/null
  fi

  sudo tee /etc/systemd/system/caddy.service >/dev/null <<'UNIT'
[Unit]
Description=Caddy
Documentation=https://caddyserver.com/docs/
After=network.target network-online.target
Requires=network-online.target

[Service]
Type=notify
User=caddy
Group=caddy
ExecStart=/usr/local/bin/caddy run --environ --config /etc/caddy/Caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile --force
TimeoutStopSec=5s
LimitNOFILE=1048576
PrivateTmp=true
ProtectSystem=full
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
UNIT

  sudo systemctl daemon-reload
  log "caddy systemd service installed."
}

install_caddy_from_github() {
  require_bin curl tar
  local arch url tmp
  arch="$(uname -m)"
  case "$arch" in
    x86_64)  arch=amd64 ;;
    aarch64) arch=arm64 ;;
    arm64)   arch=arm64 ;;
    *) fail_phase "Phase 1/3 Discovery" "install caddy" "unsupported arch: $arch" 10 ;;
  esac
  url="https://github.com/caddyserver/caddy/releases/download/v${CADDY_VERSION}/caddy_${CADDY_VERSION}_linux_${arch}.tar.gz"
  tmp="$(mktemp -d)"
  log "Downloading caddy v${CADDY_VERSION} for linux/${arch}..."
  curl -fsSL "$url" -o "$tmp/caddy.tar.gz"
  tar -xzf "$tmp/caddy.tar.gz" -C "$tmp" caddy
  sudo install -m 755 "$tmp/caddy" /usr/local/bin/caddy
  rm -rf "$tmp"
  log "Caddy binary installed: $(caddy version 2>&1 | head -1)"
  install_caddy_systemd_bundle
}

bs_install_caddy_if_missing() {
  if command -v caddy >/dev/null 2>&1; then
    info "caddy already installed: $(caddy version 2>&1 | head -1)"
    # Still make sure systemd unit + /etc/caddy exist (prior installs may have
    # skipped this on the github-binary fallback path).
    if [[ ! -f /etc/systemd/system/caddy.service ]] && [[ ! -f /usr/lib/systemd/system/caddy.service ]]; then
      warn "caddy binary present but no systemd unit found — installing service stack."
      install_caddy_systemd_bundle
    fi
    return 0
  fi
  log "caddy not found — installing..."

  # Strategy 1: yum/dnf + Fedora COPR (bundles binary + user + unit + /etc/caddy)
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
      log "caddy installed via ${pm}/COPR: $(caddy version 2>&1 | head -1)"
      return 0
    fi
    warn "${pm}/COPR install failed — falling back to GitHub binary + manual systemd."
  else
    warn "no yum/dnf found — falling back to GitHub binary + manual systemd."
  fi

  install_caddy_from_github
}

bs_detect_claude_bin() {
  # Use command -v as-is; do NOT resolve symlinks. Claude Code's installer
  # uses ~/.local/bin/claude → versions/X.Y.Z as the upgrade pivot.
  BS_CLAUDE_BIN="$(command -v claude || true)"
  if [[ -z "$BS_CLAUDE_BIN" ]]; then
    fail_phase "Phase 1/3 Discovery" "detect claude" \
      "claude binary not in PATH. Install Claude Code first." 10
  fi
  info "claude binary: $BS_CLAUDE_BIN"
}

bs_verify_claude_auth() {
  # Two valid auth states:
  #   1. ANTHROPIC_API_KEY exported in shell env
  #   2. ~/.claude/.credentials.json exists (OAuth flow completed)
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    info "claude auth: ANTHROPIC_API_KEY detected in environment"
    return 0
  fi
  if [[ -f "$HOME/.claude/.credentials.json" ]]; then
    info "claude auth: OAuth credentials present (~/.claude/.credentials.json)"
    return 0
  fi
  err "claude is not authenticated."
  err "  Run one of:"
  err "    claude          # then follow OAuth flow"
  err "    export ANTHROPIC_API_KEY=sk-ant-...   # add to your shell rc"
  err "  then re-run ./start.sh --bootstrap"
  exit 10
}

bs_verify_tools() {
  log "Verifying tool invocations..."
  if ! go version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "go version" "go is installed but broken" 10
  fi
  if ! node --version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "node --version" "node is installed but broken" 10
  fi
  if ! pnpm --version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "pnpm --version" "pnpm is installed but broken" 10
  fi
  if ! pm2 --version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "pm2 --version" "pm2 is installed but broken" 10
  fi
  if ! openssl version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "openssl version" "openssl is installed but broken" 10
  fi
  if ! caddy version >/dev/null 2>&1; then
    fail_phase "Phase 1/3 Discovery" "caddy version" "caddy is installed but broken" 10
  fi
  bs_verify_claude_auth
  log "  ✓ all tools invocable"
}

bs_detect_public_ip() {
  # IMDSv2 first (AL2023 default), then external lookup.
  local token ip=""
  token="$(curl -fsS -X PUT --max-time 2 \
    "http://169.254.169.254/latest/api/token" \
    -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null || true)"
  if [[ -n "$token" ]]; then
    ip="$(curl -fsS --max-time 2 \
      -H "X-aws-ec2-metadata-token: $token" \
      http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || true)"
  fi
  if [[ -z "$ip" ]]; then
    ip="$(curl -fsS --max-time 3 https://api.ipify.org 2>/dev/null || true)"
  fi
  BS_PUBLIC_IP="${ip//[[:space:]]/}"
  if [[ -n "$BS_PUBLIC_IP" ]]; then
    info "public IP: $BS_PUBLIC_IP"
  else
    warn "could not auto-detect public IP (not on EC2, or offline)"
  fi
}

bs_collect_inputs() {
  log "Please answer a few questions before configuration..."
  echo

  info "1/4 WORK_DIR — which repo should Claude jobs operate on?"
  info "      Default = self-hosting ($ROOT)"
  BS_WORK_DIR="$(prompt 'WORK_DIR' "$ROOT")"
  echo

  info "2/4 Caddy hostname — public DNS name for HTTPS"
  local host_default=""
  if [[ -n "$BS_PUBLIC_IP" ]]; then
    host_default="${BS_PUBLIC_IP}.nip.io"
    info "      Suggested: $host_default (nip.io wildcard DNS — real Let's Encrypt cert)"
  else
    info "      No default — supply a hostname you own or one under nip.io"
  fi
  BS_HOSTNAME="$(prompt 'Caddy hostname' "$host_default")"
  echo

  info "3/4 Public URLs"
  local nextauth_default="http://localhost:${WEB_PORT}"
  [[ -n "$BS_HOSTNAME" ]] && nextauth_default="https://${BS_HOSTNAME}"
  BS_NEXTAUTH_URL="$(prompt 'NEXTAUTH_URL (public URL of Next.js app)' "$nextauth_default")"
  BS_BACKEND_URL="$(prompt 'BACKEND_URL (server-side only)' "http://localhost:${API_PORT}")"
  echo

  info "4/4 Admin credentials"
  info "      Default: admin / Password@123"
  local use_defaults
  use_defaults="$(prompt 'Use default credentials?' 'Y')"
  if [[ "${use_defaults,,}" =~ ^y ]]; then
    BS_ADMIN_USER="admin"
    BS_ADMIN_PASS="Password@123"
    warn "USING DEFAULT ADMIN CREDENTIALS — change password after first login!"
  else
    BS_ADMIN_USER="$(prompt 'Admin username' 'admin')"
    while true; do
      BS_ADMIN_PASS="$(prompt_secret 'Admin password (min 8 chars)')"
      [[ ${#BS_ADMIN_PASS} -ge 8 ]] && break
      warn "Password too short (min 8). Try again."
    done
  fi
  echo

  info "Review your choices:"
  cat <<REVIEW
    WORK_DIR      = $BS_WORK_DIR
    Hostname      = $BS_HOSTNAME
    NEXTAUTH_URL  = $BS_NEXTAUTH_URL
    BACKEND_URL   = $BS_BACKEND_URL
    Admin user    = $BS_ADMIN_USER
    Admin pass    = $BS_ADMIN_PASS
    DB path       = $BS_PROD_DB_PATH
    CLAUDE_BIN    = $BS_CLAUDE_BIN
REVIEW
  echo
  local confirm
  confirm="$(prompt 'Proceed with these settings?' 'Y')"
  [[ "${confirm,,}" =~ ^y ]] || { err "Aborted by operator."; exit 10; }
}

bs_verify_inputs() {
  [[ -d "$BS_WORK_DIR" ]] || \
    fail_phase "Phase 1/3 Discovery" "verify WORK_DIR" \
      "WORK_DIR must exist: $BS_WORK_DIR" 10
  [[ -n "$BS_HOSTNAME" ]] || \
    fail_phase "Phase 1/3 Discovery" "verify hostname" "hostname cannot be empty" 10
  [[ ${#BS_ADMIN_PASS} -ge 8 ]] || \
    fail_phase "Phase 1/3 Discovery" "verify admin password" \
      "password too short (min 8 chars)" 10
}

bs_wipe_env_files() {
  warn "--force: wiping backend/.env and frontend/.env.local"
  rm -f backend/.env frontend/.env.local
}

discover_phase() {
  log "══ Phase 1/3: Discovery ══"
  [[ "$BS_FORCE" == "1" ]] && bs_wipe_env_files
  bs_detect_os
  bs_install_pnpm_if_missing
  bs_install_caddy_if_missing
  require_bin go node pm2 claude openssl pnpm caddy
  bs_detect_claude_bin
  bs_verify_tools
  bs_detect_public_ip
  bs_collect_inputs
  bs_verify_inputs
  checkpoint "Phase 1/3 Discovery"
}

# ═════════════════════════════════════════════════════════════════════════════
#   PHASE 2 — CONFIGURATION (writes only, no builds)
# ═════════════════════════════════════════════════════════════════════════════

bs_gen_secrets() {
  BS_JWT_SECRET="$(openssl rand -hex 32)"
  BS_AUTH_SECRET="$(openssl rand -base64 32)"
  info "generated JWT_SECRET (64 hex) and AUTH_SECRET (32 bytes base64)"
}

bs_write_backend_env() {
  if [[ -f backend/.env ]]; then
    warn "backend/.env already exists — keeping it."
    return 0
  fi
  [[ -f backend/.env.example ]] || \
    fail_phase "Phase 2/3 Configuration" "write backend/.env" \
      "backend/.env.example not found" 20

  log "Writing backend/.env..."
  cp backend/.env.example backend/.env
  sed -i.bak "s|^JWT_SECRET=.*|JWT_SECRET=${BS_JWT_SECRET}|" backend/.env
  sed -i.bak "s|^WORK_DIR=.*|WORK_DIR=${BS_WORK_DIR}|" backend/.env
  sed -i.bak "s|^CLAUDE_BIN=.*|CLAUDE_BIN=${BS_CLAUDE_BIN}|" backend/.env
  sed -i.bak "s|^DB_PATH=.*|DB_PATH=${BS_PROD_DB_PATH}|" backend/.env
  rm -f backend/.env.bak
  log "  ✓ backend/.env (JWT, WORK_DIR=${BS_WORK_DIR}, CLAUDE_BIN, DB_PATH)"
}

bs_write_frontend_env() {
  if [[ -f frontend/.env.local ]]; then
    warn "frontend/.env.local already exists — keeping it."
    return 0
  fi
  [[ -f frontend/.env.local.template ]] || \
    fail_phase "Phase 2/3 Configuration" "write frontend/.env.local" \
      "frontend/.env.local.template not found" 20

  log "Writing frontend/.env.local..."
  cp frontend/.env.local.template frontend/.env.local
  sed -i.bak "s|^AUTH_SECRET=.*|AUTH_SECRET=${BS_AUTH_SECRET}|" frontend/.env.local
  sed -i.bak "s|^NEXTAUTH_SECRET=.*|NEXTAUTH_SECRET=${BS_AUTH_SECRET}|" frontend/.env.local
  sed -i.bak "s|^NEXTAUTH_URL=.*|NEXTAUTH_URL=${BS_NEXTAUTH_URL}|" frontend/.env.local
  sed -i.bak "s|^BACKEND_URL=.*|BACKEND_URL=${BS_BACKEND_URL}|" frontend/.env.local
  sed -i.bak 's|^NEXT_PUBLIC_BACKEND_URL=.*|NEXT_PUBLIC_BACKEND_URL=|' frontend/.env.local
  rm -f frontend/.env.local.bak
  log "  ✓ frontend/.env.local (NEXTAUTH_URL=${BS_NEXTAUTH_URL})"
}

bs_generate_caddyfile() {
  local template="${SCRIPT_DIR}/Caddyfile.template"
  local output="/etc/caddy/Caddyfile"
  if [[ ! -f "$template" ]]; then
    fail_phase "Phase 2/3 Configuration" "generate Caddyfile" \
      "Caddyfile.template not found at $template" 20
  fi
  log "Generating Caddyfile from template (hostname=${BS_HOSTNAME})..."
  sed -e "s/{{HOSTNAME}}/${BS_HOSTNAME}/g" \
      -e "s/{{API_PORT}}/${API_PORT}/g" \
      -e "s/{{WEB_PORT}}/${WEB_PORT}/g" \
      "$template" | sudo tee "$output" >/dev/null
  # Capture stderr so the actual caddy error is visible to the operator
  # instead of a generic "failed validation" message.
  local validate_out
  if ! validate_out="$(sudo caddy validate --config "$output" 2>&1)"; then
    err "caddy validate output:"
    echo "$validate_out" >&2
    fail_phase "Phase 2/3 Configuration" "caddy validate" \
      "${output} failed validation (see output above)" 20
  fi
  log "  ✓ Caddyfile generated and validated"
}

bs_verify_configuration() {
  [[ -f backend/.env ]] || \
    fail_phase "Phase 2/3 Configuration" "verify backend/.env" "file missing after write" 20
  [[ -f frontend/.env.local ]] || \
    fail_phase "Phase 2/3 Configuration" "verify frontend/.env.local" "file missing after write" 20
  [[ -f /etc/caddy/Caddyfile ]] || \
    fail_phase "Phase 2/3 Configuration" "verify /etc/caddy/Caddyfile" "not installed" 20
  [[ -d /var/lib/claudemote ]] || \
    fail_phase "Phase 2/3 Configuration" "verify /var/lib/claudemote" "directory missing" 20
  log "  ✓ all config artifacts present"
}

configure_phase() {
  log "══ Phase 2/3: Configuration ══"
  bs_gen_secrets
  bs_write_backend_env
  bs_write_frontend_env
  bs_generate_caddyfile
  ensure_dirs
  bs_verify_configuration
  checkpoint "Phase 2/3 Configuration"
}

# ═════════════════════════════════════════════════════════════════════════════
#   PHASE 3 — BUILD & START (destructive / stateful)
# ═════════════════════════════════════════════════════════════════════════════

bs_create_admin_user() {
  log "Creating admin user '${BS_ADMIN_USER}'..."
  (
    cd backend \
      && ADMIN_USERNAME="$BS_ADMIN_USER" ADMIN_PASSWORD="$BS_ADMIN_PASS" \
         go run ./cmd/create-admin
  ) || fail_phase "Phase 3/3 Build & Start" "create admin user" \
        "go run ./cmd/create-admin failed — check logs above" 30
}

bs_cleanup_pm2_orphans() {
  # Only deletes pm2 processes named EXACTLY 'api' or 'web' — legacy names
  # from earlier deploys that coexist with the new claudemote-* names and
  # can hold ports ${API_PORT}/${WEB_PORT}.
  local name
  for name in api web; do
    if pm2 describe "$name" >/dev/null 2>&1; then
      warn "deleting legacy pm2 process: $name"
      pm2 delete "$name" >/dev/null 2>&1 || true
    fi
  done
  pm2 save >/dev/null 2>&1 || true
}

bs_reload_caddy() {
  log "Enabling and starting caddy..."
  sudo systemctl enable --now caddy
  sudo systemctl reload caddy
  sleep 1
  if ! systemctl is-active --quiet caddy; then
    fail_phase "Phase 3/3 Build & Start" "reload caddy" \
      "caddy is not active — check: sudo systemctl status caddy" 30
  fi
  log "  ✓ caddy active"
}

bs_verify_runtime() {
  log "Verifying runtime..."
  sleep 2  # give pm2 processes a moment to bind ports

  if ! curl -fsS "http://localhost:${API_PORT}/api/health" >/dev/null 2>&1; then
    fail_phase "Phase 3/3 Build & Start" "backend health" \
      "curl http://localhost:${API_PORT}/api/health failed" 30
  fi
  log "  ✓ backend /api/health responding"

  if ! curl -fsI "http://localhost:${WEB_PORT}" >/dev/null 2>&1; then
    fail_phase "Phase 3/3 Build & Start" "frontend" \
      "curl http://localhost:${WEB_PORT} failed" 30
  fi
  log "  ✓ frontend responding on :${WEB_PORT}"

  # Soft check — first-run cert issuance can take 30-60s.
  if curl -fsS --max-time 10 "https://${BS_HOSTNAME}/api/health" >/dev/null 2>&1; then
    log "  ✓ https://${BS_HOSTNAME}/api/health responding (cert issued)"
  else
    warn "  ⚠ https://${BS_HOSTNAME}/api/health not responding yet"
    warn "    First-run Let's Encrypt cert issuance can take 30-60s."
    warn "    Retry later: curl -v https://${BS_HOSTNAME}/api/health"
  fi
}

# Submits one trivial job via the API and polls for completion. Soft —
# warns on failure, doesn't bail. Enabled with --smoke-test flag.
bs_verify_end_to_end() {
  log "Running end-to-end smoke test..."
  local token job_id status tries=0
  # Log in to get a JWT
  token="$(curl -fsS -X POST "http://localhost:${API_PORT}/api/auth/login" \
    -H 'content-type: application/json' \
    -d "{\"username\":\"${BS_ADMIN_USER}\",\"password\":\"${BS_ADMIN_PASS}\"}" 2>/dev/null \
    | sed -n 's/.*"token":"\([^"]*\)".*/\1/p' || true)"
  if [[ -z "$token" ]]; then
    warn "  ⚠ smoke-test: login failed — skipping job submission"
    return 0
  fi
  log "  ✓ login OK"

  # Submit a trivial job
  job_id="$(curl -fsS -X POST "http://localhost:${API_PORT}/api/jobs" \
    -H "authorization: Bearer $token" \
    -H 'content-type: application/json' \
    -d '{"command":"echo smoke-test ok"}' 2>/dev/null \
    | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' || true)"
  if [[ -z "$job_id" ]]; then
    warn "  ⚠ smoke-test: job creation failed"
    return 0
  fi
  log "  ✓ job created: $job_id"

  # Poll for completion (max 60s)
  local job_json=""
  while (( tries < 30 )); do
    job_json="$(curl -fsS -H "authorization: Bearer $token" \
      "http://localhost:${API_PORT}/api/jobs/${job_id}" 2>/dev/null || true)"
    status="$(printf '%s' "$job_json" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
    case "$status" in
      done) log "  ✓ smoke-test job completed (status=done)"; return 0 ;;
      failed|canceled)
        warn "  ⚠ smoke-test job ended: $status"
        # Surface the stderr tail stored in the job summary so the operator
        # can diagnose the failure without installing sqlite3.
        local summary
        summary="$(printf '%s' "$job_json" | sed -n 's/.*"summary":"\([^"]*\)".*/\1/p')"
        [[ -n "$summary" ]] && warn "    summary: $summary"
        return 0 ;;
    esac
    sleep 2
    tries=$((tries + 1))
  done
  warn "  ⚠ smoke-test: job did not complete within 60s (last status: ${status:-unknown})"
}

build_and_start_phase() {
  log "══ Phase 3/3: Build & Start ══"
  build_backend
  build_frontend
  bs_create_admin_user
  bs_cleanup_pm2_orphans
  pm2_reload
  bs_reload_caddy
  bs_verify_runtime
  [[ "$BS_SMOKE_TEST" == "1" ]] && bs_verify_end_to_end
  checkpoint "Phase 3/3 Build & Start"
}

# ── Post-bootstrap manual steps (now much shorter) ───────────────────────────

print_remaining_manual_steps() {
  cat <<EOF

[claudemote] Bootstrap done. Remaining manual steps:

  1. DNS (optional if using nip.io) — A record → this EC2 public IP.
  2. EC2 security group — inbound TCP 80 and 443 open to 0.0.0.0/0.
  3. pm2 startup persistence (survives reboot):
       pm2 startup systemd -u \$USER --hp \$HOME
       # run the printed sudo command, then:
       pm2 save

EOF
}

# ═════════════════════════════════════════════════════════════════════════════
#   Dispatch (flag parser — order-independent)
# ═════════════════════════════════════════════════════════════════════════════

MODE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bootstrap)   MODE="bootstrap" ;;
    --force)       BS_FORCE=1 ;;
    --smoke-test)  BS_SMOKE_TEST=1 ;;
    -h|--help|help) MODE="help" ;;
    "") ;;
    *) err "Unknown arg: $1"; usage; exit 1 ;;
  esac
  shift
done

case "$MODE" in
  bootstrap)
    discover_phase
    configure_phase
    build_and_start_phase
    print_endpoints
    print_remaining_manual_steps
    ;;
  help)
    usage
    ;;
  *)
    if [[ "$BS_FORCE" == "1" || "$BS_SMOKE_TEST" == "1" ]]; then
      err "--force and --smoke-test only apply to --bootstrap"
      usage
      exit 1
    fi
    deploy
    ;;
esac
