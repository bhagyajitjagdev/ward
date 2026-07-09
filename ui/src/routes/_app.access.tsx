import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { PageHeader, Mono, ago, fmt } from "@/components/console"
import { cn } from "@/lib/utils"
import { api } from "@/lib/api"
import type { AccessEvent, AccessStats } from "@/lib/api"
import { useServices, useServiceNames } from "@/data/queries"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

export const Route = createFileRoute("/_app/access")({
  component: AccessLogPage,
})

// status → colour class for the badge/dot
function statusClass(s: number): string {
  if (s >= 500) return "text-red-600 dark:text-red-400 border-red-500/30 bg-red-500/10"
  if (s >= 400) return "text-amber-600 dark:text-amber-400 border-amber-500/30 bg-amber-500/10"
  if (s >= 300) return "text-muted-foreground border-border bg-muted/50"
  return "text-emerald-600 dark:text-emerald-400 border-emerald-500/30 bg-emerald-500/10"
}

function fmtMs(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms.toFixed(0)}ms`
}

function AccessLogPage() {
  const [service, setService] = useState("all")
  const [path, setPath] = useState("")
  const [statusClazz, setStatusClazz] = useState("all") // all | 2 | 3 | 4 | 5

  const svcParam = service === "all" ? undefined : service
  const { data: stats } = useQuery({
    queryKey: ["access-stats", svcParam],
    queryFn: () => api.accessStats({ service_id: svcParam }),
  })
  const { data: events, isLoading, error } = useQuery({
    queryKey: ["access-events", svcParam, path],
    queryFn: () => api.listAccessEvents({ service_id: svcParam, path: path || undefined, limit: 200 }),
  })

  const shown = (events ?? []).filter((e) => statusClazz === "all" || String(e.status)[0] === statusClazz)

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Monitor"
        title="Access Log"
        description="Recent requests through the edge (kept ~7 days). For long-term search, ship the access log to Loki/Grafana — see deploy."
      />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Tile label="Requests · 24h" value={stats ? fmt.format(stats.total) : "—"} sub="through the edge" />
        <Tile label="Error rate · 24h" value={stats ? `${errorRate(stats)}%` : "—"} sub="4xx + 5xx" tone={stats && Number(errorRate(stats)) > 5 ? "bad" : undefined} />
        <Tile label="Avg latency" value={stats ? fmtMs(stats.avg_ms) : "—"} sub="mean response" />
        <Tile label="p95 latency" value={stats ? fmtMs(stats.p95_ms) : "—"} sub="95th percentile" />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <section className="rounded-xl border bg-card lg:col-span-2">
          <PanelHead title="Traffic" hint="requests / 24h" />
          <div className="p-5">
            <TrafficChart data={stats?.series ?? []} />
          </div>
        </section>
        <section className="rounded-xl border bg-card">
          <PanelHead title="Top paths" hint="24h" />
          <div className="p-2">
            {stats && stats.top_paths.length > 0 ? (
              <ul className="divide-y">
                {stats.top_paths.slice(0, 8).map((p) => (
                  <li key={p.path} className="flex items-center justify-between gap-3 px-3 py-2 text-sm">
                    <Mono dim className="min-w-0 flex-1 truncate !text-xs">
                      {p.path}
                    </Mono>
                    <span className="shrink-0 font-mono tabular-nums text-muted-foreground">{fmt.format(p.count)}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="py-10 text-center text-sm text-muted-foreground">No traffic yet.</p>
            )}
          </div>
        </section>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <ServiceFilter value={service} onChange={setService} />
        <Select value={statusClazz} onValueChange={setStatusClazz}>
          <SelectTrigger className="w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">Any status</SelectItem>
            <SelectItem value="2">2xx · success</SelectItem>
            <SelectItem value="3">3xx · redirect</SelectItem>
            <SelectItem value="4">4xx · client err</SelectItem>
            <SelectItem value="5">5xx · server err</SelectItem>
          </SelectContent>
        </Select>
        <Input
          className="max-w-xs font-mono"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder="path prefix, e.g. /api"
        />
      </div>

      <div className="overflow-hidden rounded-xl border">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                <th className="px-4 py-2.5 font-medium">Time</th>
                <th className="px-4 py-2.5 font-medium">Method</th>
                <th className="px-4 py-2.5 font-medium">Path</th>
                <th className="px-4 py-2.5 font-medium">Status</th>
                <th className="px-4 py-2.5 font-medium">Latency</th>
                <th className="px-4 py-2.5 font-medium">Client IP</th>
                <th className="px-4 py-2.5 font-medium">Service</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {isLoading && (
                <tr>
                  <td colSpan={7} className="px-4 py-3.5">
                    <Skeleton className="h-6 w-full" />
                  </td>
                </tr>
              )}
              {error && (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-sm text-red-500">
                    Couldn't load the access log.
                  </td>
                </tr>
              )}
              {events && shown.length === 0 && (
                <tr>
                  <td colSpan={7} className="py-16 text-center text-sm text-muted-foreground">
                    No requests match. Traffic shows up once the edge is serving.
                  </td>
                </tr>
              )}
              {shown.map((e) => (
                <Row key={e.id} e={e} />
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

function Row({ e }: { e: AccessEvent }) {
  const names = useServiceNames()
  return (
    <tr className="transition-colors hover:bg-muted/40">
      <td className="whitespace-nowrap px-4 py-2.5">
        <Mono dim className="!text-xs">
          {ago(e.ts)}
        </Mono>
      </td>
      <td className="px-4 py-2.5">
        <span className="font-mono text-[11px] font-medium text-muted-foreground">{e.method}</span>
      </td>
      <td className="max-w-[360px] px-4 py-2.5">
        <Mono className="block truncate !text-[13px]">
          {e.path}
          {e.query ? <span className="text-muted-foreground">?{e.query}</span> : null}
        </Mono>
      </td>
      <td className="px-4 py-2.5">
        <span className={cn("inline-flex items-center rounded border px-1.5 py-0.5 font-mono text-[11px]", statusClass(e.status))}>
          {e.status}
        </span>
      </td>
      <td className="whitespace-nowrap px-4 py-2.5">
        <Mono dim className="!text-xs">
          {fmtMs(e.duration_ms)}
        </Mono>
      </td>
      <td className="px-4 py-2.5">
        <Mono dim className="!text-xs">
          {e.client_ip}
        </Mono>
      </td>
      <td className="px-4 py-2.5">
        <span className="text-xs text-muted-foreground">
          {e.service_id ? (names[e.service_id] ?? "—") : "—"}
        </span>
      </td>
    </tr>
  )
}

function errorRate(s: AccessStats): string {
  if (s.total === 0) return "0"
  return (((s.status["4xx"] + s.status["5xx"]) / s.total) * 100).toFixed(1)
}

function Tile({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone?: "bad" }) {
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{label}</div>
      <div className={cn("mt-1.5 font-heading text-2xl font-semibold tabular-nums", tone === "bad" && "text-red-500")}>
        {value}
      </div>
      {sub && <div className="mt-0.5 text-xs text-muted-foreground">{sub}</div>}
    </div>
  )
}

function PanelHead({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="flex items-baseline gap-2 border-b px-5 py-3.5">
      <h2 className="font-heading text-sm font-semibold">{title}</h2>
      {hint && <span className="font-mono text-[11px] text-muted-foreground">{hint}</span>}
    </div>
  )
}

function ServiceFilter({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { data: services } = useServices()
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-48">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">All services</SelectItem>
        {services?.map((s) => (
          <SelectItem key={s.id} value={s.id}>
            {s.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function TrafficChart({ data }: { data: AccessStats["series"] }) {
  if (data.length === 0) {
    return <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">No traffic yet.</div>
  }
  const max = Math.max(...data.map((d) => d.requests), 1)
  const w = 100
  const gap = 0.5
  const bw = Math.max(0.5, w / data.length - gap)
  return (
    <svg viewBox={`0 0 ${w} 40`} preserveAspectRatio="none" className="h-40 w-full">
      {data.map((d, i) => {
        const h = (d.requests / max) * 38
        const eh = (d.errors / max) * 38
        const x = i * (w / data.length)
        return (
          <g key={d.bucket}>
            <rect x={x} y={40 - h} width={bw} height={h} className="fill-primary/70" />
            {eh > 0 && <rect x={x} y={40 - eh} width={bw} height={eh} className="fill-red-500/80" />}
          </g>
        )
      })}
    </svg>
  )
}
