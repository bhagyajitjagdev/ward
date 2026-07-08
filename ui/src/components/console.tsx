import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

export type Severity = "critical" | "high" | "medium" | "low"

// Status semantics for the whole console: blue = armed/active, emerald = clean/ok,
// amber = detecting, red = threat/blocking, muted = idle.
export type Tone = "armed" | "ok" | "detecting" | "threat" | "idle"

const toneDot: Record<Tone, string> = {
  armed: "bg-primary",
  ok: "bg-emerald-500",
  detecting: "bg-amber-500",
  threat: "bg-red-500",
  idle: "bg-muted-foreground/40",
}

export function StatusDot({ tone, pulse = false, className }: { tone: Tone; pulse?: boolean; className?: string }) {
  return (
    <span className={cn("relative inline-flex size-2 shrink-0 items-center justify-center", className)}>
      {pulse && (
        <span className={cn("absolute inline-flex size-full animate-ping rounded-full opacity-60 motion-reduce:hidden", toneDot[tone])} />
      )}
      <span className={cn("relative inline-flex size-2 rounded-full", toneDot[tone])} />
    </span>
  )
}

const sevStyle: Record<Severity, string> = {
  critical: "text-red-600 dark:text-red-400 border-red-500/30 bg-red-500/10",
  high: "text-orange-600 dark:text-orange-400 border-orange-500/30 bg-orange-500/10",
  medium: "text-amber-600 dark:text-amber-400 border-amber-500/30 bg-amber-500/10",
  low: "text-muted-foreground border-border bg-muted/50",
}

// Map a raw backend severity (ModSecurity strings or 0–7 numbers) to our 4 buckets.
export function normalizeSeverity(s: string | undefined): Severity {
  const v = (s ?? "").toLowerCase().trim()
  if (["critical", "emergency", "alert", "0", "1", "2"].includes(v)) return "critical"
  if (["error", "high", "3"].includes(v)) return "high"
  if (["warning", "warn", "medium", "notice", "4", "5"].includes(v)) return "medium"
  return "low"
}

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded border px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase tracking-wider",
        sevStyle[severity],
      )}
    >
      {severity}
    </span>
  )
}

// A monospace technical value — the console's signature treatment for IPs, hosts, rule IDs, paths.
export function Mono({ children, className, dim }: { children: ReactNode; className?: string; dim?: boolean }) {
  return <span className={cn("font-mono text-[13px]", dim && "text-muted-foreground", className)}>{children}</span>
}

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4 border-b pb-5">
      <div className="space-y-1.5">
        {eyebrow && (
          <div className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground">{eyebrow}</div>
        )}
        <h1 className="font-heading text-2xl font-semibold tracking-tight">{title}</h1>
        {description && <p className="max-w-2xl text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}

// Relative time helper, e.g. "2m ago", "3h ago".
export function ago(iso: string): string {
  const s = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000))
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

// Forward duration, e.g. "24h", "3d" — for expiries.
export function until(iso: string): string {
  const s = Math.floor((new Date(iso).getTime() - Date.now()) / 1000)
  if (s <= 0) return "expired"
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h`
  return `${Math.floor(h / 24)}d`
}

export const fmt = new Intl.NumberFormat("en-US")

// Placeholder for screens still being designed.
export function Soon({ note }: { note: string }) {
  return (
    <div className="flex min-h-[300px] flex-col items-center justify-center rounded-xl border border-dashed bg-card/40 px-6 text-center">
      <div className="font-mono text-[11px] uppercase tracking-[0.22em] text-muted-foreground">Being designed</div>
      <p className="mt-2 max-w-sm text-sm text-muted-foreground">{note}</p>
    </div>
  )
}
