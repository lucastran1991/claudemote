// Status badge with variant colors per job lifecycle state.
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { JobStatus } from "@/types/api"

const STATUS_STYLES: Record<JobStatus, string> = {
  pending:   "bg-[rgba(186,117,23,0.2)] text-[#fac775] border-[rgba(186,117,23,0.3)]",
  running:   "bg-[rgba(127,119,221,0.2)] text-[#afa9ec] border-[rgba(127,119,221,0.3)] animate-[nexus-pulse_1.4s_infinite]",
  done:      "bg-[rgba(29,158,117,0.2)] text-[#5dcaa5] border-[rgba(29,158,117,0.3)]",
  failed:    "bg-[rgba(216,90,48,0.2)] text-[#f0997b] border-[rgba(216,90,48,0.3)]",
  cancelled: "bg-[rgba(255,255,255,0.08)] text-[rgba(255,255,255,0.45)] border-[rgba(255,255,255,0.12)]",
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
