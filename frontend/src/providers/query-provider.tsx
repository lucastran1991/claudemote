"use client"

import { QueryClientProvider } from "@tanstack/react-query"
import { getQueryClient } from "@/lib/query-client"

// Provides React Query context to the app tree
export function QueryProvider({ children }: { children: React.ReactNode }) {
  const queryClient = getQueryClient()
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}
