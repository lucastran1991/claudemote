import { auth } from "./auth"

// NEXT_PUBLIC_BACKEND_URL is intentionally empty in production:
// Caddy reverse-proxies /api/* to the backend on the same origin,
// so relative paths work without CORS. Set it only in local dev (.env.local).
const API_BASE = process.env.NEXT_PUBLIC_BACKEND_URL ?? ""

/**
 * Typed server-side fetch wrapper for the Go backend API.
 * Attaches the JWT from the active NextAuth session automatically.
 * Throws an Error with status on non-2xx responses.
 *
 * NOTE: This runs server-side (Server Components / Route Handlers).
 * For client-side mutations, call Next.js Server Actions that use this internally.
 */
export async function apiFetch<T = unknown>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const session = await auth()

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(session?.accessToken
        ? { Authorization: `Bearer ${session.accessToken}` }
        : {}),
      ...options.headers,
    },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: "Request failed" })) as Record<string, string>
    throw new Error(body.error ?? body.message ?? `API error: ${res.status}`)
  }

  // Backend wraps payloads in {"data": <payload>}. Unwrap if present.
  // Endpoints that don't use the wrapper (e.g. /api/health) are returned as-is.
  const body = (await res.json()) as { data: T } | T
  if (body && typeof body === "object" && "data" in body) {
    return (body as { data: T }).data
  }
  return body as T
}
