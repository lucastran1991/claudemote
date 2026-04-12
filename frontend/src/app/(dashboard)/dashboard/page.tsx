import { redirect } from "next/navigation"

// /dashboard → redirect to /dashboard/jobs (phase 06 will add the jobs list page)
export default function DashboardIndexPage() {
  redirect("/dashboard/jobs")
}
