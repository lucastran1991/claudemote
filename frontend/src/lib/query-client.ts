import { QueryClient } from "@tanstack/react-query"

// Singleton browser client — avoids recreating on every render
let browserQueryClient: QueryClient | undefined = undefined

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // 5 min stale time reduces redundant refetches
        staleTime: 5 * 60 * 1000,
      },
    },
  })
}

export function getQueryClient() {
  // Server: always create a new instance (no shared state between requests)
  if (typeof window === "undefined") return makeQueryClient()
  // Browser: reuse singleton
  if (!browserQueryClient) browserQueryClient = makeQueryClient()
  return browserQueryClient
}
