# claudemote

Remote Claude Code controller — submit and monitor Claude Code tasks on an EC2 instance from any browser or iOS device.

## Stack

- **Backend**: Go — HTTP API, job queue, SSE streaming, SQLite
- **Frontend**: Next.js — job submission UI, live log streaming
- **Proxy**: Caddy — HTTPS termination, SSE passthrough, reverse proxy
- **Process manager**: pm2 — two processes (`claudemote-api`, `claudemote-web`)

## Local Development

```bash
cp backend/.env.example backend/.env        # fill in JWT_SECRET etc.
cp frontend/.env.local.template frontend/.env.local
make dev                                    # starts both servers without pm2
```

API: `http://localhost:8080` | Web: `http://localhost:3000`

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

# 3. Create env files from templates
cp backend/.env.example backend/.env
#    Edit backend/.env — set JWT_SECRET, ADMIN_PASSWORD_HASH, DB_PATH, WORK_DIR
cp frontend/.env.local.template frontend/.env.local
#    Edit frontend/.env.local if needed

# 3a. Set AUTH_SECRET — required by NextAuth v5 (must be in shell env before ./start.sh)
export AUTH_SECRET="$(openssl rand -base64 32)"
#     To persist across reboots, add to /etc/environment or pm2 startup env:
#     echo "AUTH_SECRET=$AUTH_SECRET" | sudo tee -a /etc/environment

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
./start.sh          # rebuilds backend + frontend, reloads pm2
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

**`backend/.env`** (never commit this file):

| Variable | Description |
|----------|-------------|
| `JWT_SECRET` | Random 32+ char string for signing tokens |
| `ADMIN_PASSWORD_HASH` | bcrypt hash of admin password |
| `DB_PATH` | SQLite file path (default: `/var/lib/claudemote/claudemote.db`) |
| `WORK_DIR` | Repo path Claude Code operates inside |
| `PORT` | API listen port (default: 8080) |

**`frontend/.env.local`** (never commit this file):

| Variable | Description |
|----------|-------------|
| `AUTH_SECRET` | Random 32+ char string for NextAuth v5 JWT signing — generate with `openssl rand -base64 32` |
| `NEXTAUTH_SECRET` | Alias kept for NextAuth v4 compatibility — set to same value as `AUTH_SECRET` |
| `NEXTAUTH_URL` | Full public URL of the Next.js app (e.g. `https://claudemote.example.com`) |
| `BACKEND_URL` | Internal URL for server-side Next.js → API calls |
| `NEXT_PUBLIC_BACKEND_URL` | Public URL (leave empty for same-origin via Caddy) |

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
`WORK_DIR` is set to, via Claude Code running with `bypassPermissions`.
Treat the admin password like a root credential:

- Use a strong random password.
- Rotate it if you suspect compromise (`make create-admin` resets it).
- Do not share it — each user who needs access should have a separate account (v2 roadmap).
- The EC2 instance itself should have restricted SSH access (key-based, no password auth).
