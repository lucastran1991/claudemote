# Phase 05 — Frontend Scaffold (Next.js + NextAuth)

## Context
- Depends on: phase-01-backend-scaffold (API contract for auth)
- Brainstorm: ../reports/brainstorm-260412-2316-remote-claude-code-controller.md
- Reference: /Users/mac/studio/playground/frontend

## Overview
**Priority:** P0
**Status:** pending
**Effort:** M

Next.js 16 App Router frontend cloned from the playground layout: pnpm + Tailwind + shadcn/ui + React Query v5 + NextAuth v5 + React Hook Form + Zod. Login page wired to backend. Dashboard shell with nav. No job pages yet (those are phase 06).

## Key insights
- Playground has a working NextAuth Credentials → backend JWT flow. Copy verbatim, adjust the backend URL.
- Use route groups `(auth)` and `(dashboard)` per playground convention.
- `src/lib/api-client.ts` typed fetch wrapper attaches JWT from session automatically — reuse.
- Skip user-management pages (users/admin screens from playground) — claudemote is single-admin.

## Requirements
**Functional**
- `pnpm dev` runs on port 3000, proxies API calls to backend :8080.
- `/login` renders credentials form (username + password) via shadcn + RHF + Zod.
- On submit → NextAuth Credentials provider → backend `/api/auth/login` → session stores JWT.
- `/dashboard` protected — unauth'd redirected to `/login`.
- Logout clears session.

**Non-functional**
- `pnpm build` passes (no type errors, no eslint errors-as-errors).
- Dark mode via `next-themes`.
- Mobile-first Tailwind breakpoints.

## Architecture
```
frontend/
  package.json
  next.config.ts
  tailwind.config.ts
  tsconfig.json
  components.json              # shadcn registry
  src/
    app/
      layout.tsx               # root layout w/ providers
      page.tsx                 # redirect to /dashboard or /login
      (auth)/
        layout.tsx
        login/page.tsx
      (dashboard)/
        layout.tsx             # nav shell (Jobs, New Job, Logout)
        page.tsx               # /dashboard → redirect /dashboard/jobs
      api/auth/[...nextauth]/route.ts
    components/
      ui/                      # shadcn primitives
      nav-bar.tsx
    lib/
      api-client.ts            # typed fetch wrapper, attaches JWT
      auth.ts                  # NextAuth config (Credentials)
      schemas/login-schema.ts  # Zod
    providers/
      session-provider.tsx
      query-provider.tsx
      theme-provider.tsx
    types/
      api.ts                   # hand-written Job type (v1)
```

## Related code files
**Create:** all files above.
**Modify:** none.
**Delete (from playground copy-over):** any `user-management`, `admin-users`, `profile` pages unrelated to single-admin.

## Implementation steps
1. `cd /Users/mac/studio/claudemote && mkdir frontend && cd frontend && pnpm init`.
2. Add deps (match playground versions):
   - `next`, `react`, `react-dom`
   - `next-auth@beta` (v5)
   - `@tanstack/react-query`
   - `tailwindcss`, `postcss`, `autoprefixer`
   - `zod`, `react-hook-form`, `@hookform/resolvers`
   - `next-themes`
   - `lucide-react`
   - dev: `typescript`, `@types/*`, `eslint`, `eslint-config-next`
3. `pnpm dlx shadcn@latest init` → configure style, base color, etc. Install primitives: button, input, label, card, badge, table, textarea, select, skeleton, dialog, sonner (toast).
4. `src/providers/query-provider.tsx`, `session-provider.tsx`, `theme-provider.tsx` — lift from playground.
5. `src/app/layout.tsx` — wrap with SessionProvider + QueryProvider + ThemeProvider.
6. `src/lib/auth.ts` — NextAuth v5 config:
   ```ts
   import NextAuth from "next-auth"
   import Credentials from "next-auth/providers/credentials"

   export const { handlers, signIn, signOut, auth } = NextAuth({
     providers: [
       Credentials({
         credentials: { username: {}, password: {} },
         async authorize(c) {
           const res = await fetch(`${process.env.BACKEND_URL}/api/auth/login`, {
             method: "POST",
             headers: {"Content-Type":"application/json"},
             body: JSON.stringify({username: c.username, password: c.password}),
           })
           if (!res.ok) return null
           const {token, username} = await res.json()
           return { id: username, name: username, accessToken: token }
         }
       })
     ],
     callbacks: {
       jwt({token, user}) { if (user) token.accessToken = user.accessToken; return token },
       session({session, token}) { session.accessToken = token.accessToken; return session },
     },
     pages: { signIn: "/login" },
     session: { strategy: "jwt" },
   })
   ```
7. `src/app/api/auth/[...nextauth]/route.ts` → export `handlers` GET/POST.
8. `src/lib/api-client.ts`:
   ```ts
   import { auth } from "./auth"
   export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
     const session = await auth()
     const res = await fetch(`${process.env.NEXT_PUBLIC_BACKEND_URL}${path}`, {
       ...init,
       headers: {
         "Content-Type": "application/json",
         ...(session?.accessToken ? {Authorization: `Bearer ${session.accessToken}`} : {}),
         ...init?.headers,
       },
     })
     if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
     return res.json()
   }
   ```
9. `src/lib/schemas/login-schema.ts`:
   ```ts
   import { z } from "zod"
   export const loginSchema = z.object({
     username: z.string().min(1),
     password: z.string().min(1),
   })
   ```
10. `src/app/(auth)/login/page.tsx` — shadcn Card w/ form, RHF + zodResolver, calls `signIn("credentials", {...})`.
11. `src/app/(dashboard)/layout.tsx` — middleware-style auth check via `const session = await auth(); if (!session) redirect("/login")`. Nav bar with Jobs / New Job / Logout links (phase 06 fills in actual pages).
12. `src/types/api.ts` — hand-write the `Job` and `JobLog` TypeScript types matching backend schema.
13. `.env.local` template: `NEXTAUTH_URL`, `NEXTAUTH_SECRET`, `BACKEND_URL`, `NEXT_PUBLIC_BACKEND_URL`.
14. `pnpm build` → passes.

## Todo list
- [x] pnpm init + deps (match playground versions)
- [x] Tailwind + shadcn init
- [x] Providers (session, query, theme)
- [x] NextAuth config → backend login
- [x] `app/api/auth/[...nextauth]/route.ts`
- [x] `lib/api-client.ts` w/ JWT attach
- [x] `lib/schemas/login-schema.ts`
- [x] `(auth)/login/page.tsx`
- [x] `(dashboard)/layout.tsx` w/ auth guard + nav shell
- [x] `types/api.ts` hand-written types
- [x] `.env.local` template
- [x] `pnpm build` passes

## Success criteria
1. `pnpm dev` starts on :3000.
2. `/login` renders form, submitting with valid admin creds lands on `/dashboard`.
3. Direct visit to `/dashboard` while logged out → redirect to `/login`.
4. Session JWT visible in requests via `api-client.ts` (verified via DevTools Network tab).
5. Logout button clears session, redirects to `/login`.
6. Dark mode toggle works.
7. `pnpm build` clean.

## Risks
| Risk | Mitigation |
|---|---|
| NextAuth v5 breaking changes vs playground | Pin exact same version as playground's `package.json` |
| `authorize` fetch runs server-side — localhost vs prod URL | Use `BACKEND_URL` env var, NOT `NEXT_PUBLIC_*` for server-side call |
| shadcn init picks different conventions | `components.json` copied from playground verbatim |
| CORS between :3000 and :8080 in dev | Backend allows `http://localhost:3000` origin via gin cors middleware (add in phase 01 if missed) |

## Security
- `NEXTAUTH_SECRET` required, fails build if missing.
- JWT never written to localStorage — session cookie only.
- Credentials form has no "register" link — single admin, out-of-band creation via CLI.

## Next steps
Phase 06 — Jobs list, detail (with EventSource SSE), and new-job form.
