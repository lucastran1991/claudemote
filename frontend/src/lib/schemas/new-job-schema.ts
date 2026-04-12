import { z } from "zod"

export const JOB_MODELS = [
  "claude-sonnet-4-6",
  "claude-opus-4-6",
  "claude-haiku-4-5-20251001",
] as const

export type JobModel = (typeof JOB_MODELS)[number]

export const newJobSchema = z.object({
  command: z
    .string()
    .min(1, "Command is required")
    .max(10000, "Command must be under 10 000 characters"),
  model: z.enum(JOB_MODELS),
})

export type NewJobFormData = z.infer<typeof newJobSchema>
