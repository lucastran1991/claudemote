import { mergeProps } from "@base-ui/react/merge-props"
import { useRender } from "@base-ui/react/use-render"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "group/badge inline-flex h-5 w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-4xl border border-transparent px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-all focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 [&>svg]:pointer-events-none [&>svg]:size-3!",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground [a]:hover:bg-primary/80",
        secondary:
          "bg-secondary text-secondary-foreground [a]:hover:bg-secondary/80",
        destructive:
          "bg-destructive/10 text-destructive focus-visible:ring-destructive/20 dark:bg-destructive/20 dark:focus-visible:ring-destructive/40 [a]:hover:bg-destructive/20",
        outline:
          "border-border text-foreground [a]:hover:bg-muted [a]:hover:text-muted-foreground",
        ghost:
          "hover:bg-muted hover:text-muted-foreground dark:hover:bg-muted/50",
        link: "text-primary underline-offset-4 hover:underline",
        // Nexus color variants
        purple:
          "bg-[rgba(127,119,221,0.2)] text-[#afa9ec] border border-[rgba(127,119,221,0.3)]",
        teal: "bg-[rgba(29,158,117,0.2)] text-[#5dcaa5] border border-[rgba(29,158,117,0.3)]",
        coral:
          "bg-[rgba(216,90,48,0.2)] text-[#f0997b] border border-[rgba(216,90,48,0.3)]",
        amber:
          "bg-[rgba(186,117,23,0.2)] text-[#fac775] border border-[rgba(186,117,23,0.3)]",
        pink: "bg-[rgba(212,83,126,0.2)] text-[#ed93b1] border border-[rgba(212,83,126,0.3)]",
        gray: "bg-[rgba(255,255,255,0.08)] text-[rgba(255,255,255,0.45)] border border-[rgba(255,255,255,0.12)]",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant = "default",
  dot = false,
  pill = false,
  render,
  children,
  ...props
}: useRender.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & {
    dot?: boolean
    pill?: boolean
  }) {
  // Prepend a 5px circle indicator matching the text color when dot=true
  const content = dot ? (
    <>
      <span className="inline-block w-[5px] h-[5px] rounded-full bg-current" />
      {children}
    </>
  ) : (
    children
  )

  return useRender({
    defaultTagName: "span",
    props: mergeProps<"span">(
      {
        className: cn(
          badgeVariants({ variant }),
          pill && "rounded-[20px] px-2.5",
          className
        ),
      },
      { ...props, children: content }
    ),
    render,
    state: { slot: "badge", variant },
  })
}

export { Badge, badgeVariants }
