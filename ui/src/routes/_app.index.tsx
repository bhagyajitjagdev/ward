import { createFileRoute, Link } from "@tanstack/react-router"
import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { ArrowUpRight } from "lucide-react"
import { cn } from "@/lib/utils"
import { PageHeader, StatusDot, SeverityBadge, Mono, ago, fmt, normalizeSeverity } from "@/components/console"
import type { Tone } from "@/components/console"
import { api } from "@/lib/api"
import type { Overview } from "@/lib/api"
import { useServices } from "@/data/queries"

export const Route = createFileRoute("/_app/")({
  component: OverviewPage,
})

function OverviewPage() {
  const { data: overview } = useQuery({ queryKey: ["overview"], queryFn: api.overview })
  const { data: services } = useServices()
  const { data: triggers } = useQuery({ queryKey: ["top-triggers", 5], queryFn: () => api.topTriggers({ limit: 5 }) })
  const { data: events } = useQuery({ queryKey: ["waf-events", 6], queryFn: () => api.listWafEvents({ limit: 6 }) })

  const detByService = useMemo(() => {
    const m: Record<string, number> = {}
    for (const b of overview?.by_service ?? []) m[b.service_id] = b.detections_24h
    return m
  }, [overview])

  const enabled = (services ?? []).filter((s) => s.enabled)

  return (
    <div className="space-y-8">
      <PageHeader
        eyebrow="Security edge"
        title="Overview"
        description="What's reaching the edge right now, and how it's holding."
        actions={
          <div className="flex items-center gap-2 rounded-md border bg-card px-3 py-2 font-mono text-xs">
            <StatusDot tone="detecting" /> Engine · DetectionOnly
          </div>
        }
      />

      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border lg:grid-cols-4">
        <Metric label="Detections · 24h" value={overview ? fmt.format(overview.detections_24h) : "—"} sub="rule matches" tone="detecting" />
        <Metric label="Blocked · 24h" value={overview ? fmt.format(overview.blocked_24h) : "—"} sub="interrupted by the WAF" tone="threat" />
        <Metric label="Active IP blocks" value={overview ? String(overview.active_blocks) : "—"} sub="denied at the edge" />
        <Metric
          label="Protected services"
          value={overview ? `${overview.waf_services}/${overview.services}` : "—"}
          sub="WAF armed / total"
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Panel className="lg:col-span-2" title="Detection activity" hint="last 24h">
          <ActivityChart data={overview?.activity ?? []} />
        </Panel>
        <Panel title="Edge posture" hint={overview ? `${overview.waf_services}/${overview.services} armed` : undefined}>
          {enabled.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">No services yet.</p>
          ) : (
            <ul className="-my-1 divide-y">
              {enabled.map((s) => {
                const det = detByService[s.id] ?? 0
                const tone: Tone = !s.waf_enabled ? "idle" : det > 50 ? "detecting" : "armed"
                return (
                  <li key={s.id} className="flex items-center gap-3 py-2.5">
                    <StatusDot tone={tone} />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{s.name}</div>
                      <Mono dim className="block truncate !text-xs">
                        {s.public_hostname}
                      </Mono>
                    </div>
                    <div className="text-right">
                      <div className="font-mono text-sm tabular-nums">{det}</div>
                      <div className="font-mono text-[10px] uppercase tracking-wide text-muted-foreground">det</div>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </Panel>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <Panel title="Top triggers" hint="what to tune first" action={<PanelLink to="/top-triggers" />}>
          {triggers && triggers.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">Nothing's tripping the WAF.</p>
          ) : (
            <div className="space-y-3.5">
              {(triggers ?? []).slice(0, 5).map((t) => {
                const max = triggers?.[0]?.hits ?? 1
                return (
                  <div key={`${t.rule_id}-${t.path}`} className="space-y-1.5">
                    <div className="flex items-center justify-between gap-2 text-sm">
                      <div className="flex min-w-0 items-center gap-2">
                        <SeverityBadge severity={normalizeSeverity(t.severity)} />
                        <Mono className="shrink-0">{t.rule_id}</Mono>
                        <Mono dim className="truncate">
                          {t.path}
                        </Mono>
                      </div>
                      <Mono className="shrink-0 tabular-nums">{t.hits}</Mono>
                    </div>
                    <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                      <div className="h-full rounded-full bg-primary/80" style={{ width: `${(t.hits / max) * 100}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </Panel>

        <Panel title="Recent detections" hint="live" action={<PanelLink to="/waf-events" />}>
          {events && events.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">No detections yet.</p>
          ) : (
            <ul className="-my-1 divide-y">
              {(events ?? []).slice(0, 6).map((e) => (
                <li key={e.id} className="flex items-center gap-2.5 py-2.5 text-sm">
                  <SeverityBadge severity={normalizeSeverity(e.severity)} />
                  <Mono className="shrink-0">{e.rule_id}</Mono>
                  <Mono dim className="min-w-0 flex-1 truncate !text-xs">
                    {e.method} {e.path}
                  </Mono>
                  <Mono dim className="hidden shrink-0 !text-xs sm:inline">
                    {e.client_ip}
                  </Mono>
                  <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{ago(e.ts)}</span>
                </li>
              ))}
            </ul>
          )}
        </Panel>
      </div>
    </div>
  )
}

function Metric({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone?: Tone }) {
  return (
    <div className="bg-card p-5">
      <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className="mt-2 font-heading text-3xl font-semibold tabular-nums">{value}</div>
      {sub && (
        <div
          className={cn(
            "mt-1 text-xs",
            tone === "threat" ? "text-red-500" : tone === "detecting" ? "text-amber-500" : "text-muted-foreground",
          )}
        >
          {sub}
        </div>
      )}
    </div>
  )
}

function Panel({
  title,
  hint,
  action,
  className,
  children,
}: {
  title: string
  hint?: string
  action?: React.ReactNode
  className?: string
  children: React.ReactNode
}) {
  return (
    <section className={cn("rounded-xl border bg-card", className)}>
      <div className="flex items-center justify-between gap-2 border-b px-5 py-3.5">
        <div className="flex items-baseline gap-2">
          <h2 className="font-heading text-sm font-semibold">{title}</h2>
          {hint && <span className="font-mono text-[11px] text-muted-foreground">{hint}</span>}
        </div>
        {action}
      </div>
      <div className="p-5">{children}</div>
    </section>
  )
}

function PanelLink({ to }: { to: "/top-triggers" | "/waf-events" }) {
  return (
    <Link
      to={to}
      className="flex items-center gap-0.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
    >
      All <ArrowUpRight className="size-3" />
    </Link>
  )
}

function ActivityChart({ data }: { data: Overview["activity"] }) {
  const w = 760
  const h = 200
  const pad = 8
  if (data.length === 0) {
    return <div className="flex h-44 items-center justify-center text-sm text-muted-foreground">No activity yet.</div>
  }
  const max = Math.max(...data.map((d) => d.detections), 1)
  const x = (i: number) => (i / Math.max(data.length - 1, 1)) * w
  const y = (v: number) => h - pad - (v / max) * (h - 2 * pad)
  const line = (k: "detections" | "blocked") =>
    data.map((d, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(d[k]).toFixed(1)}`).join(" ")
  const area = `${line("detections")} L${w},${h} L0,${h} Z`

  return (
    <div>
      <svg viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="h-44 w-full text-primary">
        <defs>
          <linearGradient id="detGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0.28" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
          </linearGradient>
        </defs>
        <path d={area} fill="url(#detGrad)" />
        <path d={line("detections")} fill="none" stroke="currentColor" strokeWidth="2" vectorEffect="non-scaling-stroke" />
        <path
          d={line("blocked")}
          fill="none"
          className="stroke-red-500"
          strokeWidth="1.5"
          strokeDasharray="3 3"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="mt-3 flex items-center gap-4 font-mono text-[11px] text-muted-foreground">
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-2 w-3 rounded-sm bg-primary" /> Detected
        </span>
        <span className="flex items-center gap-1.5">
          <span className="inline-block w-3 border-t-2 border-dashed border-red-500" /> Blocked
        </span>
        <span className="ml-auto">peak {max}/h</span>
      </div>
    </div>
  )
}
