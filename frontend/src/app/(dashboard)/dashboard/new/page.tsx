"use client"

// /dashboard/new — centered new-job form, mobile-first layout.

import { NewJobForm } from "@/components/jobs/new-job-form"

export default function NewJobPage() {
  return (
    <div className="mx-auto max-w-2xl">
      <div className="rounded-xl border border-white/10 bg-black/20 backdrop-blur-xl p-6 space-y-6">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold text-white">New job</h1>
          <p className="text-sm text-white/50">
            Describe a task for Claude Code to execute in the configured working directory.
          </p>
        </div>
        <NewJobForm />
      </div>
    </div>
  )
}
