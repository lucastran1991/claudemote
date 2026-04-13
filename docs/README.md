# claudemote Documentation

Complete documentation for the Remote Claude Code Controller.

## Quick Navigation

### Getting Started
- **New to claudemote?** Start with [project-overview-pdr.md](./project-overview-pdr.md) for project goals, architecture, and design decisions
- **Want to deploy?** Read [deployment-guide.md](./deployment-guide.md) for bootstrap and deployment procedures
- **Want to understand how it works?** See [system-architecture.md](./system-architecture.md) for system components and data flow

### Development & Code
- **Setting up for development?** Read [code-standards.md](./code-standards.md) for codebase structure, conventions, and development workflow
- **Deep technical dive?** See [codebase-summary.md](./codebase-summary.md) for package organization, file locations, and code patterns

## Document Overview

### 1. project-overview-pdr.md
**Product overview, requirements, and design decisions.**

Contains:
- Project summary and use case
- Functional and non-functional requirements matrix
- Architecture overview
- Technology stack
- Configuration management strategy
- Key design decisions (why system.cfg.json, why no Docker, why pm2, etc.)
- Success metrics and roadmap
- Verification checklist
- Environment variables appendix

**Read if:** You're new to the project or need to understand the "why" behind design choices.

### 2. system-architecture.md
**System components, processes, configuration flows, and data movement.**

Contains:
- High-level overview (user → Next.js → Go API → Claude Code)
- Deployment architecture (EC2 instance layout, process setup)
- Process architecture (pm2 processes, Caddy reverse proxy)
- Configuration management (system.cfg.json single source of truth)
- Data flow (job submission, execution, streaming)
- Security model
- Performance characteristics
- Deployment workflow

**Read if:** You need to understand how components interact, where data flows, or how to deploy.

### 3. deployment-guide.md
**Step-by-step procedures for bootstrap and recurring deploys.**

Contains:
- Prerequisites and versions
- First deploy (3-phase bootstrap: discover, configure, build & start)
- Recurring deploys
- Configuration changes
- Troubleshooting guide (organized by phase/issue)
- Make targets reference
- Monitoring procedures
- Advanced topics (custom work directory, force bootstrap, smoke test)

**Read if:** You're deploying to EC2 or need to troubleshoot an issue.

### 4. code-standards.md
**Codebase structure, naming conventions, and development patterns.**

Contains:
- Codebase layout (backend/, frontend/, docs/)
- Configuration management (system.cfg.json vs .env files)
- Backend patterns (Go packages, error handling, testing)
- Frontend patterns (Next.js App Router, server components, auth)
- Process management (pm2, Caddy)
- Development workflow (local dev, testing, deploy)
- Code quality requirements
- Git conventions
- Glossary

**Read if:** You're contributing to the codebase or need to understand coding patterns.

### 5. codebase-summary.md
**Technical deep-dive into codebase, file locations, and code patterns.**

Contains:
- Backend entry point and key packages
- Frontend entry point and routes
- Configuration architecture (system.cfg.json → ecosystem.config.cjs → pm2 → processes)
- Deployment scripts (start.sh phases, Makefile targets)
- Process architecture (pm2 processes, Caddy systemd)
- Data flow walkthrough
- Secrets management (what's committed, what's not)
- Testing strategy
- Performance notes
- File structure summary

**Read if:** You need to find a specific file, understand configuration flows in detail, or debug an issue.

## Key Concepts

### system.cfg.json (Single Source of Truth)
All non-secret configuration flows from this committed JSON file:
- Hostname, ports (API, web)
- Worker settings (count, model, permission mode)
- Job limits (timeout, cost, log retention)
- Paths (DB, work directory)

Read by:
- `start.sh` (via jq) — generates Caddyfile, health checks
- `ecosystem.config.cjs` — injects into pm2 process environment

**Never add secrets to system.cfg.json.**

### .env Files (Secrets Only)
Gitignored, not committed. Contains secrets:
- `backend/.env` — JWT_SECRET, admin password hash, CLAUDE_BIN
- `frontend/.env.local` — NextAuth secrets

Templates committed as `.env.example` and `.env.local.template`.

### Deployment Flow
```
start.sh (3 phases)
  └─ Phase 1: Discover (detect OS, install tools, prompt for config)
  └─ Phase 2: Configure (generate env files, Caddyfile)
  └─ Phase 3: Build & Start (compile, start pm2, verify runtime)
```

### Architecture Overview
```
iOS/Browser
    ↓ HTTPS (via Caddy)
Next.js (web UI, NextAuth)
    ↓
Go API (job queue, workers, SQLite)
    ↓
Claude Code CLI (subprocesses on work_dir)
```

## Troubleshooting Quick Links

| Issue | See |
|-------|-----|
| start.sh fails | deployment-guide.md → Troubleshooting |
| Can't reach API/web | code-standards.md → Development Workflow |
| Jobs not running | deployment-guide.md → Troubleshooting → Job execution |
| HTTPS cert not issued | deployment-guide.md → Troubleshooting → HTTPS cert |
| Claude binary not found | deployment-guide.md → Troubleshooting → Claude binary |
| Configuration questions | project-overview-pdr.md → Appendix A (Environment Variables) |

## File Structure

```
docs/
├── README.md (this file)
├── project-overview-pdr.md (project spec, requirements, design decisions)
├── system-architecture.md (components, processes, data flow)
├── deployment-guide.md (bootstrap, deploy, troubleshoot)
├── code-standards.md (codebase structure, conventions)
└── codebase-summary.md (technical deep-dive, file locations)
```

## How to Use These Docs

1. **First time?** Start with project-overview-pdr.md
2. **Need to deploy?** Go to deployment-guide.md
3. **Need to develop?** Read code-standards.md
4. **Need deep technical details?** Check codebase-summary.md or system-architecture.md
5. **Something broken?** Search in deployment-guide.md troubleshooting section

## Contributing to Docs

When making changes to code:
1. Update relevant docs sections
2. Verify system.cfg.json keys are documented
3. Check .env file examples are correct
4. Update file paths if structure changes
5. Add new troubleshooting items if you encounter issues

## Key File References

| File | Purpose | Read by |
|------|---------|---------|
| system.cfg.json | Non-secret config | start.sh (jq), ecosystem.config.cjs |
| ecosystem.config.cjs | pm2 config | pm2 at startup |
| start.sh | Deploy/bootstrap script | Operators at deploy time |
| Caddyfile.template | Reverse proxy template | start.sh Phase 2 |
| backend/.env | Backend secrets | Go server at startup |
| frontend/.env.local | Frontend secrets | Next.js at startup |

## Questions or Issues?

- Configuration questions → See project-overview-pdr.md Appendix A
- Deployment issues → See deployment-guide.md Troubleshooting
- Code structure questions → See code-standards.md
- Specific file location → See codebase-summary.md

---

**Last Updated:** 2026-04-13  
**Documentation Version:** 1.0
