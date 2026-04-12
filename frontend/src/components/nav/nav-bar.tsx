"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { signOut } from "next-auth/react"
import { LayoutDashboard, PlusSquare, LogOut } from "lucide-react"
import { cn } from "@/lib/utils"

const navItems = [
  { href: "/dashboard/jobs", label: "Jobs", icon: LayoutDashboard },
  { href: "/dashboard/new", label: "New Job", icon: PlusSquare },
]

// Top navigation bar for the dashboard shell.
// Phase 06 will activate the Jobs and New Job links with real pages.
export function NavBar() {
  const pathname = usePathname()

  return (
    <header className="sticky top-0 z-40 border-b border-white/10 bg-black/20 backdrop-blur-xl">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        {/* Brand */}
        <span className="text-sm font-semibold text-white tracking-wide">
          Claudemote
        </span>

        {/* Nav links */}
        <nav className="flex items-center gap-1">
          {navItems.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors",
                pathname === href || (href !== "/dashboard" && pathname.startsWith(href))
                  ? "bg-white/10 text-white"
                  : "text-white/60 hover:bg-white/5 hover:text-white"
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </Link>
          ))}
        </nav>

        {/* Logout */}
        <button
          onClick={() => signOut({ callbackUrl: "/login" })}
          className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm text-white/60 hover:bg-white/5 hover:text-white transition-colors"
        >
          <LogOut className="h-4 w-4" />
          Logout
        </button>
      </div>
    </header>
  )
}
