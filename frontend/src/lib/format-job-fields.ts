// Shared formatting utilities for job fields used across list table, detail header, and summary card.

/** Format cost: show 4 decimals when < $0.01, 2 decimals otherwise. */
export function formatCost(usd: number): string {
  if (usd === 0) return "$0.00"
  if (usd < 0.01) return `$${usd.toFixed(4)}`
  return `$${usd.toFixed(2)}`
}

/** Format duration in ms: under 60s → "Xs", else "Xm Ys". */
export function formatDuration(ms: number): string {
  if (ms <= 0) return "—"
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 60) return `${totalSec}s`
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  return `${min}m ${sec}s`
}

/** Truncate command to maxLen chars with ellipsis. */
export function truncateCommand(cmd: string, maxLen = 80): string {
  if (cmd.length <= maxLen) return cmd
  return cmd.slice(0, maxLen) + "…"
}

/** Relative time: "X seconds/minutes/hours/days ago". */
export function relativeTime(isoString: string): string {
  const diffMs = Date.now() - new Date(isoString).getTime()
  const diffSec = Math.round(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.round(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.round(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.round(diffHr / 24)
  return `${diffDay}d ago`
}

/** Truncate session_id to first 8 chars + "…" */
export function truncateSessionId(id: string): string {
  if (!id) return "—"
  if (id.length <= 12) return id
  return id.slice(0, 8) + "…"
}
