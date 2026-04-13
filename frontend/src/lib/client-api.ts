// Client-side API helper for browser components.
// Uses useSession() to get the JWT from NextAuth — cannot use server-side apiFetch here.
// Call via useApi() hook inside React components only.

import { useSession } from "next-auth/react"
import { useCallback } from "react"
import type { Job, CreateJobRequest } from "@/types/api"

// NEXT_PUBLIC_BACKEND_URL is empty in production (same-origin via Caddy),
// set to http://localhost:<api-port> in .env.local for dev (see system.cfg.json).
const API_BASE = process.env.NEXT_PUBLIC_BACKEND_URL ?? ""

async function clientFetch<T>(
  token: string,
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...options.headers,
    },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "Request failed" })) as Record<string, string>
    throw new Error(body.error ?? body.message ?? `API error: ${res.status}`)
  }

  // 204 No Content — return undefined
  if (res.status === 204) return undefined as T

  // Backend wraps payloads in {"data": <payload>}. Unwrap if present.
  // Endpoints that don't use the wrapper (e.g. /api/health) are returned as-is.
  const body = (await res.json()) as { data: T } | T
  if (body && typeof body === "object" && "data" in body) {
    return (body as { data: T }).data
  }
  return body as T
}

// Hook that returns typed API methods bound to the current session token.
// Must be called inside a component wrapped by SessionProvider.
export function useApi() {
  const { data: session } = useSession()
  const token = session?.accessToken ?? ""

  const listJobs = useCallback(
    () => clientFetch<Job[]>(token, "/api/jobs"),
    [token]
  )

  const getJob = useCallback(
    (id: string) => clientFetch<Job>(token, `/api/jobs/${id}`),
    [token]
  )

  const createJob = useCallback(
    (body: CreateJobRequest) =>
      clientFetch<Job>(token, "/api/jobs", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    [token]
  )

  const cancelJob = useCallback(
    (id: string) =>
      clientFetch<void>(token, `/api/jobs/${id}/cancel`, { method: "POST" }),
    [token]
  )

  return { listJobs, getJob, createJob, cancelJob, token }
}
