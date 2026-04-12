# Phase 06 — Jobs UI Pages (List + Detail + New)

## Context
- Depends on: phase-04-sse-hub-and-live-stream (SSE endpoint), phase-05-frontend-scaffold-nextauth
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md

## Overview
**Priority:** P0
**Status:** pending
**Effort:** M

Three pages: jobs list, job detail with live log stream, new-job form. Mobile-first. React Query manages list polling; `EventSource` handles live log tail with automatic reconnect + `Last-Event-ID` resume.

## Key insights
- React Query polling: fast (3s) when any job is running, slow (30s) otherwise — reduces API pressure when idle.
- `EventSource` cannot set headers — JWT via `?token=` query param, or rely on NextAuth session cookie if same-origin through Caddy. Production goes through Caddy → same origin → cookie works. Dev (localhost:3000 → :8080) needs `?token=`.
- Log viewer = scrollable `<pre>` with virtualized tail; for v1, a plain `<pre>` with auto-scroll-to-bottom when user hasn't scrolled up is fine (YAGNI on virtualization until a job produces >10k lines).
- Status badges use shadcn Badge with variant colors.

## Requirements
**Functional**
- `/dashboard/jobs` — table of recent jobs, paginated, auto-refreshing.
- `/dashboard/jobs/[id]` — detail view with header (status, model, cost, duration, cancel button), live log stream, final summary when done.
- `/dashboard/new` — command textarea + model select + submit → redirect to detail page.
- Cancel button on running jobs calls `POST /api/jobs/:id/cancel`.
- Status badge color: pending=gray, running=blue-pulse, done=green, failed=red, cancelled=amber.

**Non-functional**
- Mobile: full-width textarea, touch-size submit button, no horizontal scroll on iPhone SE width (375px).
- Auto-scroll log tail unless user has scrolled up (sticky-bottom pattern).
- EventSource reconnects with exponential backoff; stops trying after 5 failures and shows banner.

## Architecture
```
src/app/(dashboard)/
  jobs/
    page.tsx                   # list
    [id]/page.tsx              # detail + live log
  new/
    page.tsx                   # form

src/components/
  job-status-badge.tsx
  job-list-table.tsx
  job-detail-header.tsx
  job-log-viewer.tsx           # EventSource + sticky-bottom <pre>
  new-job-form.tsx

src/lib/
  hooks/
    use-jobs.ts                # useQuery list w/ dynamic refetchInterval
    use-job.ts                 # useQuery single, invalidates on SSE events
  schemas/
    new-job-schema.ts          # Zod
```

## Related code files
**Create:** all files above.
**Modify:**
- `src/app/(dashboard)/layout.tsx` — ensure nav links to `/dashboard/jobs` and `/dashboard/new`
- `src/types/api.ts` — add `Job`, `JobLog`, `JobStatus` types
- `src/lib/api-client.ts` — add job-specific helpers: `listJobs`, `getJob`, `createJob`, `cancelJob`

## Implementation steps
1. `src/types/api.ts`:
   ```ts
   export type JobStatus = "pending"|"running"|"done"|"failed"|"cancelled"
   export type JobModel = "claude-sonnet-4-6"|"claude-opus-4-6"|"claude-haiku-4-5-20251001"
   export interface Job {
     id: string
     command: string
     model: JobModel
     status: JobStatus
     summary: string
     session_id: string
     duration_ms: number
     total_cost_usd: number
     num_turns: number
     is_error: boolean
     stop_reason: string
     exit_code: number | null
     created_at: string
     started_at: string | null
     finished_at: string | null
   }
   ```
2. `src/lib/api-client.ts` helpers:
   ```ts
   export const listJobs = () => apiFetch<Job[]>("/api/jobs")
   export const getJob = (id: string) => apiFetch<Job>(`/api/jobs/${id}`)
   export const createJob = (body: {command: string; model: JobModel}) =>
     apiFetch<Job>("/api/jobs", {method:"POST", body: JSON.stringify(body)})
   export const cancelJob = (id: string) =>
     apiFetch<void>(`/api/jobs/${id}/cancel`, {method:"POST"})
   ```
3. `src/lib/hooks/use-jobs.ts`:
   ```ts
   export function useJobs() {
     return useQuery({
       queryKey: ["jobs"],
       queryFn: listJobs,
       refetchInterval: (q) => {
         const anyActive = q.state.data?.some(j => j.status==="running"||j.status==="pending")
         return anyActive ? 3000 : 30000
       },
     })
   }
   ```
4. `src/components/job-status-badge.tsx` — mapping object → `<Badge variant={...}>`.
5. `src/components/job-list-table.tsx` — shadcn Table with columns: status (badge), command (truncated), model, cost, duration, created. Rows link to `/dashboard/jobs/${id}`.
6. `src/app/(dashboard)/jobs/page.tsx`:
   ```tsx
   "use client"
   export default function JobsPage() {
     const { data, isLoading } = useJobs()
     return <JobListTable jobs={data ?? []} loading={isLoading} />
   }
   ```
7. `src/components/job-log-viewer.tsx`:
   - Accept `jobId` prop.
   - `useEffect` → open `new EventSource('/api/jobs/${jobId}/stream?token=' + encodeURIComponent(token))`.
   - `onmessage` → push event data into a `lines: string[]` state (keep last 5000).
   - Track `lastEventId` from each event.
   - Sticky-bottom logic: track `isAtBottom` ref, auto-scroll only if true.
   - Reconnect backoff: on error, close + retry with 1s, 2s, 4s, 8s, 16s; after 5 fails show banner.
   - Cleanup on unmount.
8. `src/components/job-detail-header.tsx` — status badge, model, cost, duration, session_id (truncated), cancel button (if running). Cancel mutation invalidates `jobs` query.
9. `src/app/(dashboard)/jobs/[id]/page.tsx`:
   ```tsx
   "use client"
   export default function JobDetailPage({params}: {params: {id: string}}) {
     const { data: job } = useQuery({
       queryKey: ["job", params.id],
       queryFn: () => getJob(params.id),
       refetchInterval: (q) =>
         q.state.data?.status==="running" ? 2000 : false,
     })
     if (!job) return <JobDetailSkeleton />
     return (
       <div className="space-y-4">
         <JobDetailHeader job={job} />
         <JobLogViewer jobId={job.id} />
         {isTerminal(job.status) && <JobSummaryCard job={job} />}
       </div>
     )
   }
   ```
10. `src/lib/schemas/new-job-schema.ts`:
    ```ts
    export const newJobSchema = z.object({
      command: z.string().min(1, "command required").max(10000),
      model: z.enum(["claude-sonnet-4-6","claude-opus-4-6","claude-haiku-4-5-20251001"]),
    })
    ```
11. `src/components/new-job-form.tsx` — RHF + Zod, textarea + select, submit → `createJob` mutation → `router.push('/dashboard/jobs/' + newJob.id)`.
12. `src/app/(dashboard)/new/page.tsx` — renders the form centered, mobile-first.
13. Nav bar links: Jobs, New, theme toggle, logout.

## Todo list
- [x] types/api.ts complete
- [x] api-client.ts job helpers
- [x] use-jobs hook w/ dynamic interval
- [x] job-status-badge
- [x] job-list-table
- [x] jobs page
- [x] job-log-viewer (EventSource + sticky-bottom + backoff)
- [x] job-detail-header w/ cancel
- [x] jobs/[id] page
- [x] new-job-form (RHF + Zod)
- [x] new page
- [x] nav bar updated
- [x] mobile check on iPhone SE width
- [x] pnpm build clean

## Success criteria
1. Jobs list shows recent jobs, badges correct, auto-refreshes every 3s while any running, 30s when idle.
2. New job form submits, redirects to detail page, log viewer immediately streams lines.
3. Cancel button on running job terminates it; UI updates within 2-3s via polling + SSE close.
4. Mid-stream browser refresh → detail page reconnects SSE with `Last-Event-ID`, history preserved.
5. On iPhone SE (375px), no horizontal scroll; textarea and submit button full-width; nav collapses.
6. Log viewer auto-scrolls while at bottom; if user scrolls up, auto-scroll pauses until they return.
7. After 5 failed reconnects, banner: "Stream disconnected. Click to retry."
8. `pnpm build` passes.

## Risks
| Risk | Mitigation |
|---|---|
| EventSource token in URL leaks into Caddy access logs | Same-origin via Caddy uses session cookie instead; dev uses `?token=` with warning in README |
| Log viewer re-renders on every line → perf cliff | Append to ref-backed array, `flushSync` or batch with `requestAnimationFrame` |
| `refetchInterval` fires while EventSource also updates → duplicate work | Keep both: polling catches edge cases (cancel, terminal transitions); SSE handles body |
| Mobile Safari buffers EventSource until first newline | SSE protocol uses `\n\n` terminators — standard, no workaround needed |
| Stale session cookie on JWT expiry | NextAuth refresh via `callbacks.jwt`; v1 skips refresh and shows 401 → redirect to login |

## Security
- Cancel + create endpoints are JWT-protected at backend.
- EventSource token is scoped to current session; never stored in localStorage.
- Command textarea is free-text — Claude sees anything typed; warn in UI "commands run with full shell access in WORK_DIR".

## Next steps
Phase 07 — Deploy pipeline (pm2 + Caddy + start.sh) so this runs on EC2 with HTTPS + SSE passthrough.
