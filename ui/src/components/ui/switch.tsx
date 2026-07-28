import { cn } from "@/lib/utils"

// Switch — a sharp, rectangular toggle that matches Ward's console aesthetic (no
// rounded pills). role="switch" + aria-checked keep it accessible and keyboard-driven.
export function Switch({
  checked,
  onCheckedChange,
  id,
  disabled,
  "aria-label": ariaLabel,
}: {
  checked: boolean
  onCheckedChange: (v: boolean) => void
  id?: string
  disabled?: boolean
  "aria-label"?: string
}) {
  return (
    <button
      type="button"
      role="switch"
      id={id}
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex h-5 w-9 shrink-0 items-center border bg-clip-padding transition-colors outline-none",
        "focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50",
        checked ? "border-primary bg-primary" : "border-border bg-muted",
      )}
    >
      <span
        className={cn(
          "pointer-events-none block size-3.5 transition-transform",
          checked ? "translate-x-[18px] bg-primary-foreground" : "translate-x-[2px] bg-muted-foreground",
        )}
      />
    </button>
  )
}
