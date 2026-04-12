import NextAuth from "next-auth"
import Credentials from "next-auth/providers/credentials"

// NextAuth v5 config — Credentials provider wired to Go backend JWT login.
// BACKEND_URL is server-side only (not NEXT_PUBLIC_*) so the secret never leaks to the browser.
export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [
    Credentials({
      credentials: {
        username: { label: "Username", type: "text" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        const res = await fetch(
          `${process.env.BACKEND_URL}/api/auth/login`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              username: credentials?.username,
              password: credentials?.password,
            }),
          }
        )

        if (!res.ok) return null

        // Backend returns {"data": {"token": "...jwt...", "username": "..."}}
        const body = await res.json() as { data: { token: string } } | { token: string }
        const payload = (body && typeof body === "object" && "data" in body)
          ? (body as { data: { token: string } }).data
          : (body as { token: string })
        const username = String(credentials?.username ?? "admin")
        return {
          id: username,
          name: username,
          // accessToken stored in JWT callback below
          accessToken: payload.token,
        }
      },
    }),
  ],
  session: { strategy: "jwt" },
  callbacks: {
    jwt({ token, user }) {
      if (user) {
        // Persist access token from authorize() into the JWT
        token.accessToken = (user as Record<string, unknown>).accessToken as string
      }
      return token
    },
    session({ session, token }) {
      // Expose accessToken on the session object for api-client
      session.accessToken = token.accessToken as string
      return session
    },
  },
  pages: {
    signIn: "/login",
    error: "/login",
  },
})
