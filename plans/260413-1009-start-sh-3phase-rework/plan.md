# start.sh — 3-Phase Bootstrap Rework

**Status:** pending approval
**Date:** 2026-04-13
**Author:** design session after iterative bootstrap failures
**Target file:** `/Users/mac/studio/claudemote/start.sh`
**Single phase — bash rework, no multi-file split**

---

## Context

Current `start.sh` interleaves discovery, prompting, writes, and builds. Every failure we hit this session (broken `CLAUDE_BIN`, missing caddy systemd unit, DB_PATH mismatch, password-too-short at step 9, pm2 orphans holding ports) is a variant of the same anti-pattern: **late validation on partial state**.

Rework restructures bootstrap into 3 hard-gated phases:

1. **DISCOVER** — detect, install, prompt, verify. No writes.
2. **CONFIGURE** — write all config from captured inputs. No builds, no service starts.
3. **BUILD & START** — compile, migrate, start, verify runtime.

Each phase ends with a cheap verify. Failure stops immediately with a scoped error, never leaves half-state.

---

## Design Answers

### Q1 — claude auth detection (Phase 1)

`bs_verify_claude_auth` runs during `bs_verify_tools`. Two valid states:

1. `ANTHROPIC_API_KEY` is exported in the shell env
2. `~/.claude/.credentials.json` exists (OAuth flow completed)

If neither → print clear instructions and bail with exit 10 (discovery failure).

```bash
bs_verify_claude_auth() {
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    info "claude auth: ANTHROPIC_API_KEY detected"
    return 0
  fi
  if [[ -f "$HOME/.claude/.credentials.json" ]]; then
    info "claude auth: OAuth credentials detected"
    return 0
  fi
  err "claude is not authenticated — run 'claude' (OAuth) or export ANTHROPIC_API_KEY first"
  exit 10
}
```

### Q2 — default admin credentials

Default to `admin` / `Password@123` to skip an entire round of interactive typing on fresh installs. Operator can override by answering `N` to the "use defaults?" prompt.

Flow:
- Ask `Use default admin credentials (admin / Password@123)? [Y/n]`
- Y → set both to defaults, print bold warning to change after first login
- N → prompt for username + secret prompt for password with length validation loop

**Security note:** defaults are logged as a warning, not silently accepted. The warning appears in both `bs_collect_inputs` output and the final `print_endpoints` summary.

### Q3 — DB path: hardcoded

Single script constant:

```bash
readonly BS_PROD_DB_PATH="/var/lib/claudemote/claudemote.db"
```

Written into `backend/.env` during Phase 2. `ecosystem.config.cjs` already does not set DB_PATH (previous commit `1eed6af`), so `.env` is the single source of truth.

---

## File Structure (new sections)

```
start.sh (~550 LOC target)
│
├─ Shebang + header comment
├─ set -euo pipefail
├─ ROOT = cd script dir
│
├─ ── Constants ──
│   ├─ BS_PROD_DB_PATH  (readonly)
│   └─ CADDY_VERSION    (readonly)
│
├─ ── Script globals (populated by Phase 1) ──
│   ├─ BS_OS, BS_CLAUDE_BIN, BS_PUBLIC_IP
│   ├─ BS_WORK_DIR, BS_HOSTNAME, BS_NEXTAUTH_URL, BS_BACKEND_URL
│   └─ BS_ADMIN_USER, BS_ADMIN_PASS
│
├─ ── Color + log helpers ──  (log, info, warn, err — unchanged)
│
├─ ── Primitives ──
│   ├─ require_bin
│   ├─ prompt
│   ├─ prompt_secret
│   ├─ checkpoint "phase name"
│   └─ fail_phase "phase" "step" "reason"   (exit 10/20/30)
│
├─ ── Phase 1: Discovery ──
│   ├─ bs_detect_os                       (al2023|ubuntu|macos-dev|unknown)
│   ├─ bs_install_pnpm_if_missing         (corepack → npm fallback)
│   ├─ bs_install_caddy_if_missing        (dnf+COPR → github + systemd bundle)
│   │   └─ install_caddy_systemd_bundle   (NEW: user, dir, unit, daemon-reload)
│   ├─ bs_detect_claude_bin               (command -v, no readlink)
│   ├─ bs_verify_tools                    (run --version on each, incl. claude auth)
│   │   └─ bs_verify_claude_auth          (Q1)
│   ├─ bs_detect_public_ip                (IMDSv2 → ipify, curl -fsS)
│   ├─ bs_collect_inputs                  (all prompts, Q2 defaults)
│   ├─ bs_verify_inputs                   (WORK_DIR exists, hostname non-empty, pw ≥ 8)
│   └─ discover_phase                     (composes all of above + checkpoint)
│
├─ ── Phase 2: Configuration ──
│   ├─ bs_gen_secrets                     (JWT + AUTH via openssl)
│   ├─ bs_write_backend_env               (from .env.example, sed JWT/WORK_DIR/CLAUDE_BIN/DB_PATH)
│   ├─ bs_write_frontend_env              (NEXTAUTH_URL from BS_HOSTNAME, Q2)
│   ├─ bs_write_caddyfile                 (repo Caddyfile sed hostname)
│   ├─ bs_install_caddy_site              (sudo cp → /etc/caddy/Caddyfile + validate)
│   ├─ bs_ensure_system_dirs              (/var/lib/claudemote, logs/)
│   ├─ bs_verify_configuration            (caddy validate, .env readable, dirs owned)
│   └─ configure_phase                    (composes + checkpoint)
│
├─ ── Phase 3: Build & Start ──
│   ├─ bs_build_backend                   (go build -o server ./cmd/server)
│   ├─ bs_build_frontend                  (pnpm install --frozen + pnpm build)
│   ├─ bs_create_admin_user               (ADMIN_USERNAME/ADMIN_PASSWORD env vars)
│   ├─ bs_cleanup_pm2_orphans             (pm2 delete api web — by exact name)
│   ├─ bs_reload_pm2                      (pm2 startOrReload + pm2 save)
│   ├─ bs_reload_caddy                    (systemctl enable --now + reload + is-active)
│   ├─ bs_verify_runtime                  (curl localhost + caddy active + https soft)
│   └─ build_and_start_phase              (composes + checkpoint)
│
├─ ── Recurring deploy ──  (unchanged: check_envs, ensure_dirs, build_*, pm2_reload, print_endpoints)
│
├─ ── Manual-steps helper ──  (shrunk: only DNS + EC2 SG)
│
└─ ── Dispatch ──  (unchanged surface: --bootstrap | "" | --help)
```

---

## Key Function Sketches

### `install_caddy_systemd_bundle` (fills the gap we hit)

```bash
install_caddy_systemd_bundle() {
  # Called after install_caddy_from_github drops /usr/local/bin/caddy.
  # Creates the full systemd service stack: user, dir, unit, daemon-reload.
  # Idempotent — safe to re-run.
  log "Installing caddy systemd service stack..."

  # caddy user/group
  if ! getent group caddy >/dev/null 2>&1; then
    sudo groupadd --system caddy
  fi
  if ! id caddy >/dev/null 2>&1; then
    sudo useradd --system --gid caddy \
      --create-home --home-dir /var/lib/caddy \
      --shell /usr/sbin/nologin \
      --comment "Caddy web server" caddy
  fi

  # /etc/caddy + a placeholder Caddyfile so caddy can start before Phase 2 writes the real one
  sudo mkdir -p /etc/caddy
  if [[ ! -f /etc/caddy/Caddyfile ]]; then
    echo ":80 { respond \"claudemote bootstrap placeholder\" 200 }" | sudo tee /etc/caddy/Caddyfile >/dev/null
  fi

  # systemd unit (upstream template from caddyserver/dist)
  sudo tee /etc/systemd/system/caddy.service >/dev/null <<'EOF'
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
EOF

  sudo systemctl daemon-reload
  log "caddy systemd service installed."
}
```

### `bs_collect_inputs` (single prompt block with defaults + review)

```bash
bs_collect_inputs() {
  log "Please answer a few questions before configuration..."
  echo

  info "1/5 WORK_DIR — which repo should Claude jobs operate on?"
  info "       Default = self-hosting ($ROOT)"
  BS_WORK_DIR="$(prompt 'WORK_DIR' "$ROOT")"
  echo

  info "2/5 Caddy hostname — public DNS name for HTTPS"
  local host_default=""
  [[ -n "$BS_PUBLIC_IP" ]] && host_default="${BS_PUBLIC_IP}.nip.io" \
    && info "       Suggested: $host_default (nip.io wildcard, real Let's Encrypt cert)"
  BS_HOSTNAME="$(prompt 'Caddy hostname' "$host_default")"
  echo

  info "3/5 Public URLs"
  BS_NEXTAUTH_URL="$(prompt 'NEXTAUTH_URL' "https://$BS_HOSTNAME")"
  BS_BACKEND_URL="$(prompt 'BACKEND_URL (server-side only)' 'http://localhost:8080')"
  echo

  info "4/5 Admin credentials"
  info "       Default: admin / Password@123"
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

  info "5/5 Review:"
  cat <<REVIEW
    WORK_DIR      = $BS_WORK_DIR
    Hostname      = $BS_HOSTNAME
    NEXTAUTH_URL  = $BS_NEXTAUTH_URL
    BACKEND_URL   = $BS_BACKEND_URL
    Admin user    = $BS_ADMIN_USER
    Admin pass    = $([[ "$BS_ADMIN_PASS" == "Password@123" ]] && echo '[default: Password@123]' || echo '[custom]')
    DB path       = $BS_PROD_DB_PATH
    CLAUDE_BIN    = $BS_CLAUDE_BIN
REVIEW
  echo
  local confirm
  confirm="$(prompt 'Proceed with these settings?' 'Y')"
  [[ "${confirm,,}" =~ ^y ]] || { err "Aborted by operator."; exit 10; }
}
```

### `bs_verify_runtime` (Phase 3 checkpoint)

```bash
bs_verify_runtime() {
  log "Verifying runtime..."
  sleep 2  # give pm2 processes a moment to bind ports

  # Hard checks — bail on failure
  if ! curl -fsS "http://localhost:8080/api/health" >/dev/null 2>&1; then
    fail_phase "3/3" "backend health check" "curl http://localhost:8080/api/health failed"
  fi
  log "  ✓ backend /api/health responding"

  if ! curl -fsI "http://localhost:3000" >/dev/null 2>&1; then
    fail_phase "3/3" "frontend check" "curl http://localhost:3000 failed"
  fi
  log "  ✓ frontend responding on :3000"

  if ! systemctl is-active --quiet caddy; then
    fail_phase "3/3" "caddy check" "systemctl is-active caddy → inactive"
  fi
  log "  ✓ caddy active"

  # Soft check — first-run cert issuance can take 30-60s
  if curl -fsS --max-time 10 "https://${BS_HOSTNAME}/api/health" >/dev/null 2>&1; then
    log "  ✓ https://${BS_HOSTNAME}/api/health responding (cert issued)"
  else
    warn "  ⚠ https://${BS_HOSTNAME}/api/health not responding yet"
    warn "    First-run Let's Encrypt cert issuance can take 30-60s."
    warn "    Retry in 1 minute: curl -v https://${BS_HOSTNAME}/api/health"
  fi
}
```

### `bs_cleanup_pm2_orphans` (targeted by name)

```bash
bs_cleanup_pm2_orphans() {
  # Only deletes pm2 processes named exactly 'api' or 'web' — legacy names
  # from earlier deploys that coexist with the new claudemote-* names and
  # can hold ports 8080/3000.
  local name
  for name in api web; do
    if pm2 describe "$name" >/dev/null 2>&1; then
      warn "Deleting legacy pm2 process: $name"
      pm2 delete "$name" >/dev/null 2>&1 || true
    fi
  done
  pm2 save >/dev/null 2>&1 || true
}
```

### Phase composition

```bash
discover_phase() {
  log "Phase 1/3: Discovery"
  bs_detect_os
  bs_install_pnpm_if_missing
  bs_install_caddy_if_missing    # includes systemd bundle on github fallback
  bs_detect_claude_bin
  bs_verify_tools                # go version, pnpm -v, claude --version, bs_verify_claude_auth
  bs_detect_public_ip
  bs_collect_inputs
  bs_verify_inputs
  checkpoint "Phase 1/3 Discovery"
}

configure_phase() {
  log "Phase 2/3: Configuration"
  bs_gen_secrets
  bs_write_backend_env
  bs_write_frontend_env
  bs_write_caddyfile
  bs_install_caddy_site
  bs_ensure_system_dirs
  bs_verify_configuration
  checkpoint "Phase 2/3 Configuration"
}

build_and_start_phase() {
  log "Phase 3/3: Build & Start"
  bs_build_backend
  bs_build_frontend
  bs_create_admin_user
  bs_cleanup_pm2_orphans
  bs_reload_pm2
  bs_reload_caddy
  bs_verify_runtime
  checkpoint "Phase 3/3 Build & Start"
}
```

### Dispatch (unchanged surface)

```bash
case "${1:-}" in
  --bootstrap)
    discover_phase
    configure_phase
    build_and_start_phase
    print_endpoints
    print_remaining_manual_steps   # shrunk: only DNS + SG left
    ;;
  "")  deploy ;;
  -h|--help|help)  usage ;;
  *)   err "Unknown arg: $1"; usage; exit 1 ;;
esac
```

---

## Bugs Closed by This Structure

| Bug from this session | How the rework kills it |
|-----------------------|-------------------------|
| CLAUDE_BIN pointed at version dir | `bs_verify_tools` runs `claude --version` before any write; `bs_detect_claude_bin` uses `command -v` result as-is (no `readlink -f`) |
| DB_PATH mismatch between .env and pm2 env | Single constant `BS_PROD_DB_PATH`, written to .env in Phase 2, pm2 env no longer sets DB_PATH (already shipped in `1eed6af`) |
| Missing `/etc/caddy/` + no systemd unit | `install_caddy_systemd_bundle` creates user, group, dir, unit, placeholder config during Phase 1 |
| pm2 orphans (`api`, `web`) holding ports | `bs_cleanup_pm2_orphans` in Phase 3 deletes by exact name |
| Password-too-short at step 9 | Validated in `bs_collect_inputs` loop before any write |
| NEXTAUTH_URL defaults to localhost | Already fixed in `1eed6af`; reconfirmed in Phase 2 |
| Re-run leaves partial state | Phase 2 writes are idempotent; Phase 1 discovery is read-only |
| claude not authenticated | `bs_verify_claude_auth` bails in Phase 1 with clear instructions |

---

## Idempotency Rules

- **Phase 1 DISCOVER** — pure read-only except for tool installs (`dnf`, `corepack`), which are themselves idempotent. Safe to re-run.
- **Phase 2 CONFIGURE** —
  - If `backend/.env` has `JWT_SECRET=change-me-...` → write it (fresh install path)
  - If `backend/.env` already has a real `JWT_SECRET` → log "keeping existing" and skip (re-run preserves secrets)
  - Same rule for `AUTH_SECRET`, `CLAUDE_BIN`, `WORK_DIR`, `DB_PATH`, `hostname`
  - `bs_install_caddy_site` always copies latest Caddyfile from repo
  - `bs_ensure_system_dirs` uses `mkdir -p`
- **Phase 3 BUILD & START** — always rebuilds (fast on no-op diff), always starts fresh pm2 state, always reloads caddy. `bs_create_admin_user` detects existing user and resets password (create-admin.go already supports this).

---

## Error Handling & Exit Codes

| Code | Meaning | When |
|------|---------|------|
| 0 | Success | Full bootstrap complete |
| 1 | Unknown arg / bad invocation | Dispatch error |
| 10 | Phase 1 Discovery failed | Missing tool, broken binary, claude not authenticated, bad input, operator abort |
| 20 | Phase 2 Configuration failed | Template missing, caddy validate failed, sed failed |
| 30 | Phase 3 Build & Start failed | Compile error, migration error, pm2 failure, runtime verify failed |

Each `fail_phase` call prints:
1. Which phase + which step
2. The reason
3. Concrete "what to do" recovery hint

---

## Testing Plan

### Pre-commit (dev machine)
- [ ] `bash -n start.sh` — syntax clean
- [ ] `shellcheck start.sh` — zero warnings
- [ ] Read through each phase, confirm no forward references to BS_* vars set later
- [ ] Diff against current start.sh for accidental removal of non-bootstrap paths

### Local dry-run (macOS dev)
- [ ] `./start.sh --help` — help text renders
- [ ] `./start.sh` (no arg, existing .env files) — deploy path unchanged
- [ ] Can't truly run `--bootstrap` on macOS because dnf/systemctl missing — rely on EC2

### Live test (EC2 13.59.195.76)
1. `cd /opt/atomiton/claudemote && git pull`
2. `rm -f backend/.env frontend/.env.local`
3. `sudo rm -rf /etc/caddy /etc/systemd/system/caddy.service`  (force github-binary path to re-bundle)
4. `sudo userdel caddy 2>/dev/null; sudo groupdel caddy 2>/dev/null` (force user recreation)
5. `pm2 delete api web claudemote-api claudemote-web 2>/dev/null`
6. `./start.sh --bootstrap`
7. Expect: interactive prompts in one block → Phase 1 green → Phase 2 green → Phase 3 green → curl health ok → https cert issuance (may need 60s retry)
8. Browser: login at `https://13-59-195-76.nip.io` with `admin` / `Password@123`
9. Submit trivial job via UI: `echo hello` → expect it to stream and complete

### Regression on recurring deploy
1. After successful bootstrap, run plain `./start.sh`
2. Expect: no prompts, fast rebuild, pm2 reload, endpoints printed, no changes to caddy or .env

---

## Risks

1. **`claude --version` behavior unknown** — may still prompt for login. Mitigation: if it prompts, swap for a file-existence check only (`~/.claude/.credentials.json` || env var).
2. **`caddy validate` syntax on AL2023** — older caddy versions use different flag form. Mitigation: pinned version 2.8.4, validated flag format.
3. **`pm2 delete api web` could delete real processes** — only if someone actually named their pm2 processes exactly `api` or `web`. Mitigation: exact-name match (not regex), clear warn log before deletion, only within `--bootstrap` path.
4. **`bs_install_caddy_site` overwrites existing /etc/caddy/Caddyfile** — intentional on bootstrap, but could clobber hand-edits. Mitigation: Phase 2 is bootstrap-only; plain `./start.sh` doesn't touch caddy config.
5. **First-run HTTPS verify flakes on cold cert** — already soft-gated, prints retry instructions.

---

## Related Code Files

**Modified:**
- `start.sh` — full bootstrap rewrite

**Read but not modified:**
- `ecosystem.config.cjs` — verify DB_PATH still absent (already fixed in 1eed6af)
- `backend/.env.example` — template source
- `frontend/.env.local.template` — template source
- `Caddyfile` — site config template
- `backend/cmd/create-admin/main.go` — confirm ADMIN_USERNAME/ADMIN_PASSWORD env var contract

**Potentially affected docs:**
- `docs/deployment-guide.md` — should document the 3-phase flow post-implementation

---

## Todo List

- [ ] 1. Add script-global `BS_*` variable declarations + `BS_PROD_DB_PATH` constant
- [ ] 2. Add `checkpoint` + `fail_phase` primitives
- [ ] 3. Implement `bs_detect_os`
- [ ] 4. Refactor `install_pnpm_if_missing` → `bs_install_pnpm_if_missing`
- [ ] 5. Refactor `install_caddy_if_missing` → `bs_install_caddy_if_missing`
- [ ] 6. **NEW** `install_caddy_systemd_bundle` — user, group, /etc/caddy, unit file, daemon-reload
- [ ] 7. Implement `bs_detect_claude_bin` (no readlink)
- [ ] 8. Implement `bs_verify_tools` including `bs_verify_claude_auth` (Q1)
- [ ] 9. Refactor `detect_ec2_public_ip` → `bs_detect_public_ip`
- [ ] 10. Implement `bs_collect_inputs` with Q2 defaults + review block
- [ ] 11. Implement `bs_verify_inputs`
- [ ] 12. Implement `bs_gen_secrets` (JWT + AUTH_SECRET)
- [ ] 13. Implement `bs_write_backend_env` (JWT/WORK_DIR/CLAUDE_BIN/DB_PATH — Q3 hardcoded)
- [ ] 14. Implement `bs_write_frontend_env` (NEXTAUTH_URL from BS_HOSTNAME)
- [ ] 15. Implement `bs_write_caddyfile`
- [ ] 16. Implement `bs_install_caddy_site` (sudo cp + `caddy validate`)
- [ ] 17. Implement `bs_ensure_system_dirs`
- [ ] 18. Implement `bs_verify_configuration`
- [ ] 19. Implement `bs_build_backend`
- [ ] 20. Implement `bs_build_frontend`
- [ ] 21. Implement `bs_create_admin_user`
- [ ] 22. Implement `bs_cleanup_pm2_orphans`
- [ ] 23. Implement `bs_reload_pm2`
- [ ] 24. Implement `bs_reload_caddy`
- [ ] 25. Implement `bs_verify_runtime`
- [ ] 26. Compose `discover_phase`, `configure_phase`, `build_and_start_phase`
- [ ] 27. Shrink `print_remaining_manual_steps` to DNS + security group
- [ ] 28. Add flag parser (loop over `$@`, set `BS_FORCE=1` / `BS_SMOKE_TEST=1`)
- [ ] 29. **NEW** `bs_wipe_env_files` — called before Phase 1 when `BS_FORCE=1`
- [ ] 30. **NEW** `bs_verify_end_to_end` — submits one trivial job via API, polls, reports (soft verify)
- [ ] 31. Change review block in `bs_collect_inputs` to echo `BS_ADMIN_PASS` plaintext (per resolution #3)
- [ ] 32. Update dispatch to call the 3 phase functions with flag handling
- [ ] 33. Remove old `bootstrap*` helper functions
- [ ] 34. Update `usage` output with new flags
- [ ] 35. `bash -n start.sh` + `shellcheck start.sh` → zero warnings
- [ ] 36. Commit with conventional message
- [ ] 37. Push to main
- [ ] 38. Live-test on EC2 with `--bootstrap --force --smoke-test`, document any deltas

---

## Success Criteria

1. `bash -n` clean, `shellcheck` clean
2. Fresh bootstrap on a wiped EC2 (no .env, no caddy config, no systemd unit, no /var/lib/claudemote) completes in one non-interactive run aside from the 5-prompt collect block
3. `curl https://<hostname>/api/health` returns `{"ok":true}` after bootstrap (possibly after 60s cert wait)
4. Browser login at `https://<hostname>` succeeds with `admin` / `Password@123`
5. Running plain `./start.sh` on the deployed box rebuilds and redeploys without prompts
6. Running `./start.sh --bootstrap` a second time on a clean run is safe (idempotent, preserves secrets)
7. Any single phase failure prints actionable recovery info and leaves the system recoverable

---

## Resolved Decisions (answered 2026-04-13)

1. **Plain `./start.sh` does NOT auto-install caddy** — bootstrap-only. Fast path stays fast.
2. **Add `--force` flag** — wipes `backend/.env` and `frontend/.env.local` before Phase 1. Usage: `./start.sh --bootstrap --force`. Logs a warning before deletion.
3. **Review block echoes password plain text** — operator explicitly approved. Acceptable since (a) still in the interactive session, (b) default `Password@123` is public anyway, (c) custom passwords are still captured via `prompt_secret` with no-echo during entry.
4. **Add `--smoke-test` flag** — runs `bs_verify_end_to_end` as Phase 3 final step. Submits one trivial job via the API (`echo hi` or similar), polls for completion, reports pass/fail. Soft verify — warn on failure, don't bail. Usage: `./start.sh --bootstrap --smoke-test` or combine `./start.sh --bootstrap --force --smoke-test`.

### Flag interaction matrix

| Command | Behavior |
|---------|----------|
| `./start.sh` | Recurring deploy (unchanged) |
| `./start.sh --bootstrap` | 3-phase first-run setup |
| `./start.sh --bootstrap --force` | Wipe .env files first, then 3-phase |
| `./start.sh --bootstrap --smoke-test` | 3-phase + end-to-end job submission |
| `./start.sh --bootstrap --force --smoke-test` | Clean slate + 3-phase + smoke test |
| `./start.sh --help` | Usage |

Flags are parsed with a simple loop; order-independent.
