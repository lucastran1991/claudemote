import * as React from "react"
import { Input as InputPrimitive } from "@base-ui/react/input"

import { cn } from "@/lib/utils"

// Convenience classes for consumers that need to apply success/error state via className
export const inputStateClasses = {
  success:
    "border-[rgba(29,158,117,0.6)] bg-[rgba(29,158,117,0.07)] shadow-[0_0_0_3px_rgba(29,158,117,0.15)]",
  error:
    "border-[rgba(216,90,48,0.6)] bg-[rgba(216,90,48,0.07)] shadow-[0_0_0_3px_rgba(216,90,48,0.15)]",
}

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-8 w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1 text-base transition-all outline-none",
        "file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground",
        "placeholder:text-muted-foreground",
        // Nexus purple focus ring
        "focus-visible:border-[rgba(127,119,221,0.6)] focus-visible:bg-[rgba(127,119,221,0.08)] focus-visible:shadow-[0_0_0_3px_rgba(127,119,221,0.15)]",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50",
        // Nexus coral error state via aria-invalid
        "aria-invalid:border-[rgba(216,90,48,0.6)] aria-invalid:bg-[rgba(216,90,48,0.07)] aria-invalid:shadow-[0_0_0_3px_rgba(216,90,48,0.15)]",
        "md:text-sm dark:bg-input/30 dark:disabled:bg-input/80",
        className
      )}
      {...props}
    />
  )
}

export { Input }
