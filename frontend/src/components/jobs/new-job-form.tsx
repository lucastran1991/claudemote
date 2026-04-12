"use client"

// New job form — RHF + Zod validation. Uses native <select> to avoid base-ui/RHF
// controller complexity. On success, redirects to the new job's detail page.

import { useRouter } from "next/navigation"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { newJobSchema, JOB_MODELS, type NewJobFormData } from "@/lib/schemas/new-job-schema"
import { useApi } from "@/lib/client-api"
import { cn } from "@/lib/utils"

const MODEL_LABELS: Record<string, string> = {
  "claude-sonnet-4-6":          "Claude Sonnet 4.6 (recommended)",
  "claude-opus-4-6":            "Claude Opus 4.6",
  "claude-haiku-4-5-20251001":  "Claude Haiku 4.5",
}

export function NewJobForm() {
  const router = useRouter()
  const { createJob } = useApi()
  const queryClient = useQueryClient()

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<NewJobFormData>({
    resolver: zodResolver(newJobSchema),
    defaultValues: { model: "claude-sonnet-4-6" },
  })

  const mutation = useMutation({
    mutationFn: (data: NewJobFormData) => createJob(data),
    onSuccess: (job) => {
      // Invalidate list so jobs page shows new entry immediately
      queryClient.invalidateQueries({ queryKey: ["jobs"] })
      router.push(`/dashboard/jobs/${job.id}`)
    },
  })

  async function onSubmit(data: NewJobFormData) {
    mutation.mutate(data)
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
      {/* Command textarea */}
      <div className="space-y-1.5">
        <label htmlFor="command" className="text-sm text-white/70">
          Command
        </label>
        <p className="text-xs text-white/40">
          Runs with full shell access in the configured working directory.
        </p>
        <textarea
          id="command"
          rows={8}
          placeholder="e.g. Refactor the auth module to use JWT refresh tokens"
          {...register("command")}
          className={cn(
            "w-full rounded-lg border bg-white/5 px-3 py-2 text-sm text-white placeholder-white/20",
            "font-mono resize-y focus:outline-none focus:ring-2 focus:ring-primary/60 transition-colors",
            errors.command ? "border-red-400" : "border-white/10"
          )}
        />
        {errors.command && (
          <p className="text-xs text-red-400">{errors.command.message}</p>
        )}
      </div>

      {/* Model select */}
      <div className="space-y-1.5">
        <label htmlFor="model" className="text-sm text-white/70">
          Model
        </label>
        <select
          id="model"
          {...register("model")}
          className={cn(
            "w-full rounded-lg border bg-white/5 px-3 py-2 text-sm text-white",
            "focus:outline-none focus:ring-2 focus:ring-primary/60 transition-colors",
            "appearance-none cursor-pointer",
            errors.model ? "border-red-400" : "border-white/10"
          )}
        >
          {JOB_MODELS.map((m) => (
            <option key={m} value={m} className="bg-slate-900 text-white">
              {MODEL_LABELS[m] ?? m}
            </option>
          ))}
        </select>
        {errors.model && (
          <p className="text-xs text-red-400">{errors.model.message}</p>
        )}
      </div>

      {/* Server error */}
      {mutation.isError && (
        <p className="text-xs text-red-400">
          {mutation.error instanceof Error
            ? mutation.error.message
            : "Failed to create job"}
        </p>
      )}

      {/* Submit */}
      <button
        type="submit"
        disabled={isSubmitting || mutation.isPending}
        className={cn(
          "w-full rounded-lg bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground",
          "hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-primary/60",
          "disabled:opacity-50 disabled:cursor-not-allowed transition-colors min-h-[44px]"
        )}
      >
        {mutation.isPending ? "Submitting…" : "Run job"}
      </button>
    </form>
  )
}
