import { handlers } from "@/lib/auth"

// NextAuth v5 catch-all route — handles GET (session) and POST (sign-in/out)
export const { GET, POST } = handlers
