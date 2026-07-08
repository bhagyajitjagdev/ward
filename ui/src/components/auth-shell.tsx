import { ShieldCheck } from "lucide-react"

export function AuthShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      {/* brand panel — the dark command rail identity */}
      <div className="relative hidden flex-col justify-between overflow-hidden bg-sidebar p-10 text-sidebar-foreground lg:flex">
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage:
              "linear-gradient(currentColor 1px, transparent 1px), linear-gradient(90deg, currentColor 1px, transparent 1px)",
            backgroundSize: "34px 34px",
          }}
        />
        <div className="relative flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <ShieldCheck className="size-5" />
          </div>
          <span className="font-heading text-lg font-semibold">Ward</span>
        </div>

        <div className="relative space-y-4">
          <div className="font-mono text-[11px] uppercase tracking-[0.22em] text-sidebar-foreground/60">
            Security edge
          </div>
          <h1 className="font-heading text-3xl font-semibold leading-tight">
            Your edge — armed,
            <br />
            and finally legible.
          </h1>
          <p className="max-w-sm text-sm text-sidebar-foreground/70">
            A humane control plane over Caddy + Coraza. See what's hitting you, tune the WAF in one click, and roll back
            anything.
          </p>
        </div>

        <div className="relative flex items-center gap-2 font-mono text-[11px] text-sidebar-foreground/50">
          <span className="inline-block size-2 rounded-full bg-emerald-500" />
          caddy + coraza · self-hosted
        </div>
      </div>

      {/* form panel */}
      <div className="flex items-center justify-center p-6">
        <div className="w-full max-w-sm">
          <div className="mb-8 flex items-center gap-2.5 lg:hidden">
            <div className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <ShieldCheck className="size-5" />
            </div>
            <span className="font-heading text-lg font-semibold">Ward</span>
          </div>
          {children}
        </div>
      </div>
    </div>
  )
}
