"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { signIn } from "next-auth/react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { loginSchema, type LoginFormData } from "@/lib/schemas/login-schema"
import { cn } from "@/lib/utils"

// Login page — single-admin credentials form wired to NextAuth Credentials provider
export default function LoginPage() {
  const router = useRouter()
  const [serverError, setServerError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  })

  async function onSubmit(data: LoginFormData) {
    setServerError(null)
    const result = await signIn("credentials", {
      username: data.username,
      password: data.password,
      redirect: false,
    })

    if (result?.error) {
      setServerError("Invalid username or password")
      return
    }

    router.push("/dashboard")
    router.refresh()
  }

  return (
    <div className="rounded-xl border border-white/10 bg-black/30 backdrop-blur-xl p-8 shadow-2xl space-y-6">
      {/* Header */}
      <div className="space-y-1 text-center">
        <h1 className="text-2xl font-bold text-white">Claudemote</h1>
        <p className="text-sm text-white/50">Sign in to continue</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        {/* Username */}
        <div className="space-y-1">
          <label htmlFor="username" className="text-sm text-white/70">
            Username
          </label>
          <input
            id="username"
            type="text"
            autoComplete="username"
            {...register("username")}
            className={cn(
              "w-full rounded-lg border bg-white/5 px-3 py-2 text-sm text-white placeholder-white/30",
              "focus:outline-none focus:ring-2 focus:ring-primary/60",
              errors.username ? "border-red-400" : "border-white/10"
            )}
            placeholder="admin"
          />
          {errors.username && (
            <p className="text-xs text-red-400">{errors.username.message}</p>
          )}
        </div>

        {/* Password */}
        <div className="space-y-1">
          <label htmlFor="password" className="text-sm text-white/70">
            Password
          </label>
          <input
            id="password"
            type="password"
            autoComplete="current-password"
            {...register("password")}
            className={cn(
              "w-full rounded-lg border bg-white/5 px-3 py-2 text-sm text-white placeholder-white/30",
              "focus:outline-none focus:ring-2 focus:ring-primary/60",
              errors.password ? "border-red-400" : "border-white/10"
            )}
            placeholder="••••••••"
          />
          {errors.password && (
            <p className="text-xs text-red-400">{errors.password.message}</p>
          )}
        </div>

        {/* Server error */}
        {serverError && (
          <p className="text-xs text-red-400 text-center">{serverError}</p>
        )}

        {/* Submit */}
        <button
          type="submit"
          disabled={isSubmitting}
          className={cn(
            "w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground",
            "hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-primary/60",
            "disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          )}
        >
          {isSubmitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  )
}
