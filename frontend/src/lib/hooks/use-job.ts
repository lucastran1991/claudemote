// React Query hook for a single job. Polls every 2s while running, stops when terminal.

import { useQuery } from "@tanstack/react-query"
import { useApi } from "@/lib/client-api"

const TERMINAL_STATUSES = new Set(["done", "failed", "cancelled"])

export function useJob(id: string) {
  const { getJob } = useApi()

  return useQuery({
    queryKey: ["job", id],
    queryFn: () => getJob(id),
    refetchInterval: (query) => {
      const status = query.state.data?.status
      if (!status || TERMINAL_STATUSES.has(status)) return false
      return 2000
    },
    enabled: Boolean(id),
  })
}
