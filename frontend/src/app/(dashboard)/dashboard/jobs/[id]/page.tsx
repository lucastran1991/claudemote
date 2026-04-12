"use client"

// /dashboard/jobs/[id] — job detail: header, live log stream, terminal summary.

import { use } from "react"
import Link from "next/link"
import { ArrowLeft } from "lucide-react"
import { Skeleton } from "@/components/ui/skeleton"
import { useJob } from "@/lib/hooks/use-job"
import { JobDetailHeader } from "@/components/jobs/job-detail-header"
import { JobLogViewer } from "@/components/jobs/job-log-viewer"
import { JobSummaryCard } from "@/components/jobs/job-summary-card"

const TERMINAL_STATUSES = new Set(["done", "failed", "cancelled"])

function JobDetailSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-32 w-full rounded-xl" />
      <Skeleton className="h-96 w-full rounded-xl" />
    </div>
  )
}

interface PageProps {
  params: Promise<{ id: string }>
}

export default function JobDetailPage({ params }: PageProps) {
  // Next.js 16 App Router passes params as a Promise
  const { id } = use(params)
  const { data: job, isLoading, isError, error } = useJob(id)

  if (isLoading) return <JobDetailSkeleton />

  if (isError || !job) {
    return (
      <div className="space-y-4">
        <Link
          href="/dashboard/jobs"
          className="inline-flex items-center gap-1 text-sm text-white/50 hover:text-white transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to jobs
        </Link>
        <div className="rounded-lg border border-red-500/20 bg-red-500/10 px-4 py-3">
          <p className="text-sm text-red-400">
            {error instanceof Error ? error.message : "Job not found"}
          </p>
        </div>
      </div>
    )
  }

  const isTerminal = TERMINAL_STATUSES.has(job.status)

  return (
    <div className="space-y-4">
      <Link
        href="/dashboard/jobs"
        className="inline-flex items-center gap-1 text-sm text-white/50 hover:text-white transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to jobs
      </Link>

      <JobDetailHeader job={job} />
      <JobLogViewer jobId={job.id} isTerminal={isTerminal} />
      {isTerminal && <JobSummaryCard job={job} />}
    </div>
  )
}
