// Auth route group layout — centers the login card, no nav shell
export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <main className="min-h-screen flex items-center justify-center px-4 bg-gradient-to-br from-[#0f0c29] via-[#302b63] to-[#24243e]">
      <div className="w-full max-w-sm">
        {children}
      </div>
    </main>
  )
}
