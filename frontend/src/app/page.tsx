import { redirect } from "next/navigation"
import { auth } from "@/lib/auth"

// Root page: redirect authenticated users to dashboard, others to login
export default async function Home() {
  const session = await auth()

  if (session) {
    redirect("/dashboard")
  }

  redirect("/login")
}
