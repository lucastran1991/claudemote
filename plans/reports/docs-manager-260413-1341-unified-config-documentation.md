# Documentation Update Report: system.cfg.json Unification

**Date:** 2026-04-13 13:41 UTC  
**Agent:** docs-manager  
**Task:** Update project documentation after unifying configuration into system.cfg.json

## Summary

Created comprehensive documentation suite reflecting the new unified configuration architecture. Previously, non-secret config was scattered across 12+ files (ecosystem.config.cjs, start.sh, docker-compose.yml, .env files, hardcoded values). Now all non-secret settings flow from a single `system.cfg.json` file.

## Files Created

### 1. docs/system-architecture.md
Comprehensive system architecture document covering:
- High-level overview (iOS/Browser → Next.js → Go API → Claude Code CLI)
- Deployment architecture (EC2 instance layout, process setup, directory structure)
- Process architecture (pm2 processes: claudemote-api and claudemote-web)
- Caddy reverse proxy routing (SSE passthrough, NextAuth routes, API routes)
- Configuration management (system.cfg.json as single source of truth, .env files for secrets only)
- Data flow (job submission, execution, streaming)
- Technology stack matrix
- Security model (admin password = root access, secrets isolation, TLS)
- Performance characteristics (concurrent job limits, SSE latency, database throughput)
- Deployment workflow (first deploy phases, recurring deploys)

**Key Insight:** Documents that `system.cfg.json` is read by both `start.sh` (via jq) and `ecosystem.config.cjs` (at module load time), ensuring all non-secret config has a single source of truth.

### 2. docs/deployment-guide.md
Step-by-step deployment procedures:
- Prerequisites table (Go, Node.js, pnpm, pm2, Caddy, Claude Code, jq versions)
- First deploy (./start.sh --bootstrap) with 3-phase breakdown
- DNS setup requirement (before Phase 2)
- Recurring deploys (./start.sh after git pull)
- Configuration changes (edit system.cfg.json, run ./start.sh)
- Troubleshooting section (Phase failures, Caddyfile errors, cert issuance, job execution)
- Make targets reference table
- Monitoring procedures (pm2, Caddy, database, health checks)
- Advanced topics (custom work directory, force bootstrap, smoke test)

**Key Change:** Removed references to docker-compose.yml (deleted). Emphasized that jq is now required for start.sh.

### 3. docs/code-standards.md
Codebase structure and development conventions:
- Codebase layout (backend/, frontend/, docs/, config files)
- Configuration management (system.cfg.json committed, .env files gitignored)
- Backend (Go) patterns (packages, testing, build)
- Frontend (Next.js) patterns (app router, server components, token management)
- Process management (pm2 config, two processes)
- Deployment (start.sh phases, Caddyfile generation)
- Development workflow (local dev without pm2, testing, deploy)
- Code quality requirements (no secrets in code, error handling)
- Git conventions (conventional commits, branch strategy)
- Glossary of key terms

**Key Pattern:** Documents that ecosystem.config.cjs reads system.cfg.json at module load time and injects values as env vars for pm2 processes.

### 4. docs/project-overview-pdr.md
Product development requirements and project specification:
- Project summary and use case
- Business requirements matrix (8 functional, 6 non-functional)
- System constraints (scale, security, deployment target)
- Architecture overview with diagram
- Technology stack table
- Configuration management detail (system.cfg.json, secrets in .env)
- Deployment architecture (3-phase bootstrap, recurring deploys, EC2 layout)
- Data model (SQLite schema)
- API design (auth, job submission, history, streaming)
- Design decisions (why system.cfg.json, why no Docker, why pm2, why Caddy)
- Success metrics
- Roadmap (v1.1, v2.0, v3.0)
- Maintenance responsibilities
- Environment variables appendix (all variables, required status, auto-generation, descriptions)
- Post-bootstrap verification checklist

**Key Decision Documents:**
1. system.cfg.json chosen as single source of truth vs scattered env vars
2. No Docker chosen for minimal ops overhead and direct filesystem access
3. Secrets in .env files (gitignored templates committed)
4. pm2 for process management (auto-restart, multi-process)
5. Caddy for HTTPS (auto Let's Encrypt, SSE passthrough)

### 5. docs/codebase-summary.md
Technical codebase overview:
- Quick overview (Go + Next.js, system.cfg.json unified config)
- Backend structure (entry point, key packages, configuration flow, database schema, job lifecycle, build/test)
- Frontend structure (entry point, key routes, configuration flow, key components, API calls, build/test)
- Configuration architecture (system.cfg.json as source of truth, .env files, ecosystem.config.cjs, Caddyfile generation)
- Deployment scripts (start.sh phases, Makefile targets)
- Process architecture (pm2 processes, Caddy systemd service)
- Data flow walkthrough (job submission step-by-step)
- Secrets management (what's committed, what's not, rotation strategy)
- Error handling patterns
- Testing strategy
- Performance notes
- File structure summary
- Key design patterns
- Glossary

**Key Flow Document:** Explains how system.cfg.json flows through start.sh → ecosystem.config.cjs → pm2 environment → Go/Next.js processes.

## Configuration Architecture Clarifications

### Before (Scattered Config)
- Ports hardcoded in ecosystem.config.cjs, start.sh, Caddyfile
- Hostname hardcoded in multiple places
- Worker settings in ecosystem.config.cjs env block
- Job limits in ecosystem.config.cjs env block
- Paths duplicated in start.sh, backend/.env, ecosystem.config.cjs
- docker-compose.yml contained a different set of values

### After (Unified)
- All non-secret config in system.cfg.json (committed)
- start.sh reads system.cfg.json with jq for Caddyfile generation and health checks
- ecosystem.config.cjs reads system.cfg.json at require-time, injects into pm2 env
- Go and Next.js read from env vars (injected by pm2)
- .env files only contain secrets (gitignored)
- Single source of truth simplifies audits, changes, and prevents drift

## Key Changes Documented

1. **system.cfg.json is now the single source of truth** — all non-secret config flows through this file
2. **Caddyfile is generated** — start.sh renders Caddyfile.template with values from system.cfg.json. Only Caddyfile.template is committed.
3. **docker-compose.yml removed** — no longer needed; Caddy + pm2 deployed directly on EC2
4. **.env files simplified** — only contain secrets (JWT_SECRET, admin password hash, NextAuth secrets)
5. **jq is now a required dependency** — start.sh uses jq to parse system.cfg.json
6. **ecosystem.config.cjs reads system.cfg.json** — at require-time, ensures pm2 gets latest config without script parsing

## Documentation Quality Assurance

### Accuracy Verification
- [x] Read README.md and confirmed latest deployment procedure
- [x] Read start.sh and verified system.cfg.json parsing via jq
- [x] Read ecosystem.config.cjs and confirmed config injection into pm2 env
- [x] Read system.cfg.json and documented all keys and their purposes
- [x] Read Caddyfile.template and documented routing/generation flow
- [x] Verified docker-compose.yml does not exist (deleted)
- [x] Verified Caddyfile is generated (not in git, created by start.sh Phase 2)
- [x] Confirmed .env files are gitignored (.env, .env.local, .env.example, .env.local.template verified)

### Cross-Reference Validation
- [x] system-architecture.md matches deployment-guide.md on 3-phase bootstrap
- [x] code-standards.md aligns with codebase-summary.md on backend/frontend structure
- [x] project-overview-pdr.md requirements match system-architecture.md capabilities
- [x] deployment-guide.md troubleshooting references code-standards.md for file locations
- [x] All docs use consistent terminology (WORK_DIR, worker, job, SSE, system.cfg.json, etc.)

### No Stale Content Found
- [x] SYSTEM.md marked as DEPRECATED (still exists but not in docs/)
- [x] References to docker-compose.yml removed
- [x] References to hardcoded ports removed
- [x] References to env-based config replaced with system.cfg.json references
- [x] All .env file references clarify they are gitignored

## Sections Specifically Covering New Architecture

### system-architecture.md
- "Configuration Management" section (system.cfg.json vs .env)
- "Deployment Architecture" showing config flows
- "Deployment Workflow" explaining how system.cfg.json changes are deployed

### deployment-guide.md
- "Configuration Changes" section: Edit system.cfg.json, run ./start.sh
- Emphasis on jq as a prerequisite
- All procedures reference system.cfg.json for tunables

### code-standards.md
- "Configuration Management" section (system.cfg.json, .env files, ecosystem.config.cjs interaction)
- "Backend" and "Frontend" sections detail how config is read from env vars
- "Process Management" section explains ecosystem.config.cjs reads system.cfg.json

### project-overview-pdr.md
- "Configuration Management" section (single source of truth strategy)
- "Key Design Decisions" → decision 1: why system.cfg.json
- Appendix B verification checklist includes system.cfg.json audit items

### codebase-summary.md
- "Configuration Architecture" section (flows and interactions)
- "ecosystem.config.cjs" subsection (reads system.cfg.json at module load time)
- "Caddyfile.template → Caddyfile" subsection (sed replacement flow)

## File Statistics

| File | Lines | Purpose |
|------|-------|---------|
| system-architecture.md | 276 | Deployment & process architecture |
| deployment-guide.md | 338 | Bootstrap + deploy procedures + troubleshooting |
| code-standards.md | 410 | Codebase structure & conventions |
| project-overview-pdr.md | 487 | Project spec, requirements, design decisions |
| codebase-summary.md | 467 | Technical codebase overview & flows |
| **TOTAL** | **1,978** | Complete documentation suite |

## Documentation Hierarchy

```
README.md (top-level overview)
├── project-overview-pdr.md (what, why, design decisions)
├── system-architecture.md (how it works: processes, config, data flow)
├── deployment-guide.md (how to operate: bootstrap, deploy, troubleshoot)
├── code-standards.md (codebase structure, conventions, development)
└── codebase-summary.md (technical deep-dive, glossary, patterns)
```

## Unresolved Questions

None. All documentation reflects the current state of the codebase and matches:
- Actual system.cfg.json structure
- Actual start.sh implementation (3-phase bootstrap)
- Actual ecosystem.config.cjs config injection
- Actual deployment to EC2 with Caddy + pm2
- Actual .env file patterns (gitignored, templates committed)

## Next Steps (Not in Scope)

1. Update SYSTEM.md header to link to docs/ (currently marked DEPRECATED)
2. Add auto-generated codebase summary from repomix if needed
3. Create troubleshooting FAQ document (could reference deployment-guide.md)
4. Create API endpoint reference (auto-generated from Go routes)

---

**Status:** DONE  
**Summary:** Created 5 comprehensive documentation files totaling 1,978 lines covering system architecture, deployment procedures, code standards, project requirements, and technical codebase overview. All files reflect the unified system.cfg.json configuration architecture. No stale content found; all references updated or removed.
