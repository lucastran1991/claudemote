import type { Metadata } from "next"
import { ThemeProvider } from "@/providers/theme-provider"
import { QueryProvider } from "@/providers/query-provider"
import { SessionProvider } from "@/providers/session-provider"
import "./globals.css"

export const metadata: Metadata = {
  title: "Claudemote",
  description: "Remote Claude Code controller",
}

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      className="h-full antialiased dark"
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col">
        <SessionProvider>
          <ThemeProvider>
            <QueryProvider>{children}</QueryProvider>
          </ThemeProvider>
        </SessionProvider>
      </body>
    </html>
  )
}
