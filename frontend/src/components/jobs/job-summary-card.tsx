"use client"

// Shown when a job reaches a terminal state. Displays final summary, stop reason,
// turns, cost, duration, and error flag.

import { formatCost, formatDuration } from "@/lib/format-job-fields"
import type { Job } from "@/types/api"

interface JobSummaryCardProps {
  job: Job
}

function SummaryRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2 border-b border-white/5 last:border-0">
      <span className="text-xs text-white/40 uppercase tracking-wide shrink-0">{label}</span>
      <span className="text-xs text-white/80 font-mono text-right">{value}</span>
    </div>
  )
}

export function JobSummaryCard({ job }: JobSummaryCardProps) {
  return (
    <div className="rounded-xl border border-white/10 bg-black/20 backdrop-blur-xl p-5 space-y-1">
      <h3 className="text-sm font-semibold text-white/80 mb-3">Summary</h3>

      {job.is_error && (
        <div className="mb-3 rounded-lg bg-red-500/10 border border-red-500/20 px-3 py-2">
          <p className="text-xs text-red-300">Job completed with errors.</p>
        </div>
      )}

      {job.summary && (
        <div className="mb-3">
          <p className="text-xs text-white/40 uppercase tracking-wide mb-1">Result</p>
          <p className="text-sm text-white/70 whitespace-pre-wrap">{job.summary}</p>
        </div>
      )}

      <SummaryRow label="Stop reason" value={job.stop_reason || "—"} />
      <SummaryRow label="Turns" value={job.num_turns} />
      <SummaryRow label="Total cost" value={formatCost(job.total_cost_usd)} />
      <SummaryRow label="Duration" value={formatDuration(job.duration_ms)} />
      {job.exit_code !== null && (
        <SummaryRow label="Exit code" value={String(job.exit_code)} />
      )}
    </div>
  )
}
