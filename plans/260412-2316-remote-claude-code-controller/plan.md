---
name: Remote Claude Code Controller (claudemote)
status: completed
created: 2026-04-12
updated: 2026-04-13
blockedBy: []
blocks: []
progress:
  phases_done: 7
  phases_total: 7
  implementation: completed
  qa: completed
  review_findings: 10 fixed (3 critical, 7 important)
---

# claudemote — Remote Claude Code Controller

Web app to queue + monitor Claude Code jobs on remote EC2 from browser/iOS.

## Source
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md
- Stack reference: /Users/mac/studio/playground

## Architecture (one-liner)
iOS/Browser → Caddy → Next.js (NextAuth) / Go API (JWT) → SQLite + worker pool → `claude -p --output-format stream-json` subprocess (cwd=WORK_DIR) → SSE hub → UI live tail.

## Locked decisions
- Auth: NextAuth JWT (reuse playground flow)
- Concurrency: worker pool, `WORKER_COUNT=2` default
- Logs: full stored in SQLite + SSE live stream
- Repo: single pinned `WORK_DIR` (v1)
- Model: `claude-sonnet-4-6` default, opus/haiku as opt-in per job
- Cost cap: `MAX_COST_PER_JOB_USD=1.00` default
- Queue: SQLite-backed w/ crash recovery on boot
- Cancel: `ctx.Cancel` → SIGTERM on process group
- CLI flags: `--output-format stream-json --permission-mode bypassPermissions --model <model>`

## Phases

| # | Phase | Status | Effort | Blocks |
|---|---|---|---|---|
| 01 | Backend scaffold (Gin+GORM+SQLite+JWT) | completed | M | 02,05 |
| 02 | Worker pool + `claude -p` subprocess runner | completed | M | 03 |
| 03 | stream-json parser + JobLog + cost/timeout guards | completed | M | 04 |
| 04 | SSE hub + live stream endpoint | completed | S | 06 |
| 05 | Frontend scaffold (Next.js + NextAuth) | completed | M | 06 |
| 06 | Jobs UI pages (list + detail + new) | completed | M | 07 |
| 07 | Deploy: pm2 + Caddy + start.sh | completed | S | — |

## Dependency chain

```
01 ─┬─► 02 ─► 03 ─► 04 ─┐
    └─► 05 ─► 06 ◄──────┴─► 07
```

## Out of v1 (deferred)
Presets, multi-repo, push notifications, session resume, structured tool-call rendering, git auto-commit.

## Top risks
1. Runaway API cost — Sonnet default + cost cap + wall-clock timeout + cancel
2. 32k-token CLAUDE.md tax on target repo — follow-up task, trim CLAUDE.md
3. SSE through Caddy — `flush_interval -1` required, tested end-to-end in phase 07

## Open follow-ups (not blocking)
- Trim `/opt/atomiton/playwright-demo/CLAUDE.md` to cut cache-creation tax
- v2 scope lock: presets, multi-repo, resume
