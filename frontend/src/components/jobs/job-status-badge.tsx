// Status badge with variant colors per job lifecycle state.
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { JobStatus } from "@/types/api"

const STATUS_STYLES: Record<JobStatus, string> = {
  pending:   "bg-slate-500/20 text-slate-300 border-slate-500/30",
  running:   "bg-blue-500/20 text-blue-300 border-blue-500/30 animate-pulse",
  done:      "bg-green-500/20 text-green-300 border-green-500/30",
  failed:    "bg-red-500/20 text-red-300 border-red-500/30",
  cancelled: "bg-amber-500/20 text-amber-300 border-amber-500/30",
}

const STATUS_LABELS: Record<JobStatus, string> = {
  pending:   "Pending",
  running:   "Running",
  done:      "Done",
  failed:    "Failed",
  cancelled: "Cancelled",
}

interface JobStatusBadgeProps {
  status: JobStatus
  className?: string
}

export function JobStatusBadge({ status, className }: JobStatusBadgeProps) {
  return (
    <Badge
      className={cn(
        "border font-medium",
        STATUS_STYLES[status] ?? STATUS_STYLES.pending,
        className
      )}
    >
      {STATUS_LABELS[status] ?? status}
    </Badge>
  )
}
