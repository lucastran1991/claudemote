"use client"

import { ThemeProvider as NextThemesProvider } from "next-themes"

// Wraps next-themes to support system/light/dark mode switching.
// The `dark` class on <html> drives Tailwind dark: variants.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="dark"
      enableSystem={false}
      disableTransitionOnChange
    >
      {children}
    </NextThemesProvider>
  )
}
