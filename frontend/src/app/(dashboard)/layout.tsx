import { redirect } from "next/navigation"
import { auth } from "@/lib/auth"
import { NavBar } from "@/components/nav/nav-bar"

// Dashboard route group layout.
// Auth guard: any unauthenticated request is redirected to /login server-side.
export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const session = await auth()

  if (!session) {
    redirect("/login")
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-[#0f0c29] via-[#302b63] to-[#24243e]">
      <NavBar />
      <main className="mx-auto max-w-7xl px-4 py-6">{children}</main>
    </div>
  )
}
