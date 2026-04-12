"use client"

// /dashboard/jobs — jobs list with auto-refresh via React Query.

import Link from "next/link"
import { useJobs } from "@/lib/hooks/use-jobs"
import { JobListTable } from "@/components/jobs/job-list-table"
import { PlusSquare } from "lucide-react"

export default function JobsPage() {
  const { data, isLoading, isError, error } = useJobs()

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-white">Jobs</h1>
        <Link
          href="/dashboard/new"
          className="flex items-center gap-2 rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          <PlusSquare className="h-4 w-4" />
          New job
        </Link>
      </div>

      {isError && (
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 px-4 py-3">
          <p className="text-sm text-red-400">
            {error instanceof Error ? error.message : "Failed to load jobs"}
          </p>
        </div>
      )}

      <JobListTable jobs={data ?? []} loading={isLoading} />
    </div>
  )
}
