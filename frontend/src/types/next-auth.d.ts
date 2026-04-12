import "next-auth"
import "next-auth/jwt"

// Augment NextAuth session and JWT types to carry the backend access token
declare module "next-auth" {
  interface Session {
    accessToken: string
  }
  interface User {
    accessToken?: string
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string
  }
}
