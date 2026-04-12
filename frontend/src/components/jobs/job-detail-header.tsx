"use client"

// Job detail header card: status, model, cost, duration, session_id, cancel button.

import { useMutation, useQueryClient } from "@tanstack/react-query"
import { JobStatusBadge } from "@/components/jobs/job-status-badge"
import { useApi } from "@/lib/client-api"
import {
  formatCost,
  formatDuration,
  truncateSessionId,
} from "@/lib/format-job-fields"
import type { Job } from "@/types/api"

interface JobDetailHeaderProps {
  job: Job
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-0.5">
      <p className="text-xs text-white/40 uppercase tracking-wide">{label}</p>
      <p className="text-sm text-white/80 font-mono">{value}</p>
    </div>
  )
}

export function JobDetailHeader({ job }: JobDetailHeaderProps) {
  const { cancelJob } = useApi()
  const queryClient = useQueryClient()

  const cancelMutation = useMutation({
    mutationFn: () => cancelJob(job.id),
    onSuccess: () => {
      // Invalidate both list and detail queries so UI updates immediately
      queryClient.invalidateQueries({ queryKey: ["jobs"] })
      queryClient.invalidateQueries({ queryKey: ["job", job.id] })
    },
  })

  const isRunning = job.status === "running"

  return (
    <div className="rounded-xl border border-white/10 bg-black/20 backdrop-blur-xl p-5 space-y-4">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <JobStatusBadge status={job.status} />
          <span className="text-white/40 text-xs font-mono">{job.id.slice(0, 8)}…</span>
        </div>

        {isRunning && (
          <button
            onClick={() => cancelMutation.mutate()}
            disabled={cancelMutation.isPending}
            className="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-1.5 text-xs text-red-300
                       hover:bg-red-500/20 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {cancelMutation.isPending ? "Cancelling…" : "Cancel job"}
          </button>
        )}
      </div>

      {cancelMutation.isError && (
        <p className="text-xs text-red-400">
          Cancel failed: {cancelMutation.error instanceof Error
            ? cancelMutation.error.message
            : "unknown error"}
        </p>
      )}

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <MetaItem label="Model" value={job.model} />
        <MetaItem label="Cost" value={formatCost(job.total_cost_usd)} />
        <MetaItem label="Duration" value={formatDuration(job.duration_ms)} />
        <MetaItem label="Session" value={truncateSessionId(job.session_id)} />
      </div>

      <div className="space-y-1">
        <p className="text-xs text-white/40 uppercase tracking-wide">Command</p>
        <p className="text-sm text-white/70 font-mono whitespace-pre-wrap break-all">
          {job.command}
        </p>
      </div>
    </div>
  )
}
