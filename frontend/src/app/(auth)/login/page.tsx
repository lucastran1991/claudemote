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
    <div className="glass-dark p-8 shadow-2xl space-y-6">
      {/* Header */}
      <div className="space-y-1 text-center">
        <h1 className="text-2xl font-bold bg-gradient-to-r from-[#afa9ec] to-[#5dcaa5] bg-clip-text text-transparent">Claudemote</h1>
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
              "w-full rounded-[10px] border bg-white/5 px-3 py-2.5 text-[13.5px] text-white placeholder-white/22 transition-all",
              "focus:outline-none focus:border-[rgba(127,119,221,0.6)] focus:bg-[rgba(127,119,221,0.08)] focus:shadow-[0_0_0_3px_rgba(127,119,221,0.15)]",
              errors.username ? "border-[rgba(216,90,48,0.6)] bg-[rgba(216,90,48,0.07)]" : "border-white/10"
            )}
            placeholder="admin"
          />
          {errors.username && (
            <p className="text-xs text-[#f0997b]">{errors.username.message}</p>
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
              "w-full rounded-[10px] border bg-white/5 px-3 py-2.5 text-[13.5px] text-white placeholder-white/22 transition-all",
              "focus:outline-none focus:border-[rgba(127,119,221,0.6)] focus:bg-[rgba(127,119,221,0.08)] focus:shadow-[0_0_0_3px_rgba(127,119,221,0.15)]",
              errors.password ? "border-[rgba(216,90,48,0.6)] bg-[rgba(216,90,48,0.07)]" : "border-white/10"
            )}
            placeholder="••••••••"
          />
          {errors.password && (
            <p className="text-xs text-[#f0997b]">{errors.password.message}</p>
          )}
        </div>

        {/* Server error */}
        {serverError && (
          <p className="text-xs text-[#f0997b] text-center">{serverError}</p>
        )}

        {/* Submit */}
        <button
          type="submit"
          disabled={isSubmitting}
          className={cn(
            "w-full rounded-[10px] bg-gradient-to-br from-[#7f77dd] to-[#534ab7] px-4 py-3 text-sm font-medium text-white",
            "hover:from-[#afa9ec] hover:to-[#7f77dd] hover:-translate-y-px hover:shadow-[0_8px_24px_rgba(127,119,221,0.35)]",
            "focus:outline-none focus:shadow-[0_0_0_3px_rgba(127,119,221,0.15)]",
            "disabled:opacity-50 disabled:cursor-not-allowed transition-all"
          )}
        >
          {isSubmitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  )
}
