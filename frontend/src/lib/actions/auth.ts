"use server"

import { signOut } from "@/lib/auth"

// Logout server action. Uses the server-side signOut from the NextAuth v5
// config export (not the client-side next-auth/react helper, which has
// reliability issues in v5 beta on Next.js 16).
//
// Invoked via <form action={logoutAction}> in client components — Next.js
// handles the server-action round-trip and honors the redirect.
export async function logoutAction() {
  await signOut({ redirectTo: "/login" })
}
