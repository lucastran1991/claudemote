"use client"

// Jobs list table — status badge, truncated command, model, cost, duration, relative time.
// Each row links to /dashboard/jobs/:id.

import Link from "next/link"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { JobStatusBadge } from "@/components/jobs/job-status-badge"
import {
  formatCost,
  formatDuration,
  truncateCommand,
  relativeTime,
} from "@/lib/format-job-fields"
import type { Job } from "@/types/api"

interface JobListTableProps {
  jobs: Job[]
  loading: boolean
}

function LoadingRows() {
  return (
    <>
      {Array.from({ length: 5 }).map((_, i) => (
        <TableRow key={i}>
          <TableCell><Skeleton className="h-4 w-16" /></TableCell>
          <TableCell><Skeleton className="h-4 w-64" /></TableCell>
          <TableCell><Skeleton className="h-4 w-32" /></TableCell>
          <TableCell><Skeleton className="h-4 w-12" /></TableCell>
          <TableCell><Skeleton className="h-4 w-12" /></TableCell>
          <TableCell><Skeleton className="h-4 w-16" /></TableCell>
        </TableRow>
      ))}
    </>
  )
}

export function JobListTable({ jobs, loading }: JobListTableProps) {
  return (
    <div className="rounded-xl border border-white/10 bg-black/20 backdrop-blur-xl overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow className="border-white/10 hover:bg-transparent">
            <TableHead className="text-white/60">Status</TableHead>
            <TableHead className="text-white/60">Command</TableHead>
            <TableHead className="text-white/60 hidden sm:table-cell">Model</TableHead>
            <TableHead className="text-white/60 hidden md:table-cell">Cost</TableHead>
            <TableHead className="text-white/60 hidden md:table-cell">Duration</TableHead>
            <TableHead className="text-white/60">Created</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading ? (
            <LoadingRows />
          ) : jobs.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={6}
                className="text-center text-white/40 py-12"
              >
                No jobs yet — create one to get started.
              </TableCell>
            </TableRow>
          ) : (
            jobs.map((job) => (
              <TableRow
                key={job.id}
                className="border-white/5 hover:bg-white/5 cursor-pointer"
              >
                <TableCell>
                  <Link href={`/dashboard/jobs/${job.id}`} className="block">
                    <JobStatusBadge status={job.status} />
                  </Link>
                </TableCell>
                <TableCell className="max-w-xs">
                  <Link
                    href={`/dashboard/jobs/${job.id}`}
                    className="block text-white/80 hover:text-white font-mono text-xs truncate"
                    title={job.command}
                  >
                    {truncateCommand(job.command)}
                  </Link>
                </TableCell>
                <TableCell className="text-white/60 text-xs hidden sm:table-cell">
                  <Link href={`/dashboard/jobs/${job.id}`} className="block">
                    {job.model}
                  </Link>
                </TableCell>
                <TableCell className="text-white/60 text-xs hidden md:table-cell">
                  <Link href={`/dashboard/jobs/${job.id}`} className="block">
                    {formatCost(job.total_cost_usd)}
                  </Link>
                </TableCell>
                <TableCell className="text-white/60 text-xs hidden md:table-cell">
                  <Link href={`/dashboard/jobs/${job.id}`} className="block">
                    {formatDuration(job.duration_ms)}
                  </Link>
                </TableCell>
                <TableCell className="text-white/40 text-xs">
                  <Link href={`/dashboard/jobs/${job.id}`} className="block">
                    {relativeTime(job.created_at)}
                  </Link>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  )
}
