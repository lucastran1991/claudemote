// React Query hook for the jobs list with dynamic refetch interval.
// Polls fast (3s) when any job is running/pending, slow (30s) when idle.

import { useQuery } from "@tanstack/react-query"
import { useApi } from "@/lib/client-api"

export function useJobs() {
  const { listJobs } = useApi()

  return useQuery({
    queryKey: ["jobs"],
    queryFn: listJobs,
    refetchInterval: (query) => {
      const anyActive = query.state.data?.some(
        (j) => j.status === "running" || j.status === "pending"
      )
      return anyActive ? 3000 : 30000
    },
  })
}
