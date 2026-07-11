import { useMemo, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { Search, ChevronRight, FilterX, Ban } from "lucide-react"
import { cn } from "@/lib/utils"
import { PageHeader, SeverityBadge, Mono, ago, normalizeSeverity } from "@/components/console"
import type { Severity } from "@/components/console"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { api, ApiError } from "@/lib/api"
import type { WafEvent } from "@/lib/api"
import { useServices, useServiceNames } from "@/data/queries"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"

export const Route = createFileRoute("/_app/waf-events")({
  component: WafEventsPage,
})

const severities: (Severity | "all")[] = ["all", "critical", "high", "medium", "low"]

function methodColor(m: string) {
  return m === "POST" ? "text-amber-500" : m === "DELETE" ? "text-red-500" : "text-emerald-500"
}

function WafEventsPage() {
  const [q, setQ] = useState("")
  const [sev, setSev] = useState<Severity | "all">("all")
  const [svc, setSvc] = useState<string>("all") // service id
  const [selected, setSelected] = useState<WafEvent | null>(null)

  const names = useServiceNames()
  const { data: services } = useServices()
  const { data: events, isLoading, error } = useQuery({
    queryKey: ["waf-events"],
    queryFn: () => api.listWafEvents({ limit: 200 }),
    refetchInterval: 5000, // live monitor: poll every 5s (react-query auto-pauses when the tab is hidden)
  })

  const filtered = useMemo(
    () =>
      (events ?? []).filter((e) => {
        if (sev !== "all" && normalizeSeverity(e.severity) !== sev) return false
        if (svc !== "all" && e.service_id !== svc) return false
        if (q) {
          const hay = `${e.rule_id} ${e.rule_msg} ${e.path} ${e.client_ip} ${e.matched_target ?? ""}`.toLowerCase()
          if (!hay.includes(q.toLowerCase())) return false
        }
        return true
      }),
    [events, q, sev, svc],
  )

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Monitor"
        title="WAF Events"
        description="Every request the WAF flagged. Search it, judge it — attack or false positive — then tune or block."
        actions={<Mono dim className="!text-xs uppercase tracking-wider">last 200</Mono>}
      />

      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[240px] flex-1">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search rule, path, IP, target…"
            className="pl-9 font-mono text-sm"
          />
        </div>
        <Select value={svc} onValueChange={setSvc}>
          <SelectTrigger className="w-[168px]">
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
        <div className="flex rounded-md border bg-card p-0.5">
          {severities.map((s) => (
            <button
              key={s}
              onClick={() => setSev(s)}
              className={cn(
                "rounded px-2.5 py-1 font-mono text-[11px] uppercase tracking-wide transition-colors",
                sev === s ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {s}
            </button>
          ))}
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Time</th>
              <th className="px-4 py-2.5 font-medium">Sev</th>
              <th className="px-4 py-2.5 font-medium">Rule</th>
              <th className="px-4 py-2.5 font-medium">Request</th>
              <th className="px-4 py-2.5 font-medium">Client</th>
              <th className="px-4 py-2.5 font-medium">Service</th>
              <th className="w-8" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading &&
              [0, 1, 2, 3].map((i) => (
                <tr key={i}>
                  <td colSpan={7} className="px-4 py-3.5">
                    <Skeleton className="h-6 w-full" />
                  </td>
                </tr>
              ))}
            {error && (
              <tr>
                <td colSpan={7} className="py-12 text-center text-sm text-red-500">
                  Couldn't load WAF events.
                </td>
              </tr>
            )}
            {events && filtered.length === 0 && (
              <tr>
                <td colSpan={7} className="py-16 text-center text-sm text-muted-foreground">
                  {events.length === 0 ? "No detections yet — the WAF hasn't flagged anything." : "No events match these filters."}
                </td>
              </tr>
            )}
            {filtered.map((e) => (
              <tr
                key={e.id}
                onClick={() => setSelected(e)}
                className="group cursor-pointer align-top transition-colors hover:bg-muted/40"
              >
                <td className="whitespace-nowrap px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(e.ts)}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  <SeverityBadge severity={normalizeSeverity(e.severity)} />
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <Mono>{e.rule_id}</Mono>
                    {e.is_anomaly_score && (
                      <span className="rounded bg-muted px-1 font-mono text-[10px] text-muted-foreground">anomaly</span>
                    )}
                  </div>
                  <div className="max-w-[260px] truncate text-xs text-muted-foreground">{e.rule_msg}</div>
                </td>
                <td className="px-4 py-3">
                  <Mono className="!text-xs">
                    <span className={methodColor(e.method)}>{e.method}</span> {e.path}
                  </Mono>
                  <div className="font-mono text-[11px] text-muted-foreground">
                    {e.matched_target || "—"} · {e.status}
                  </div>
                </td>
                <td className="whitespace-nowrap px-4 py-3">
                  <Mono className="!text-xs">{e.client_ip}</Mono>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {e.service_id ? (names[e.service_id] ?? "—") : "—"}
                  </Mono>
                </td>
                <td className="pr-3 align-middle">
                  <ChevronRight className="size-4 text-muted-foreground/40 transition-colors group-hover:text-muted-foreground" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <EventSheet event={selected} serviceName={selected?.service_id ? names[selected.service_id] : undefined} onClose={() => setSelected(null)} />
    </div>
  )
}

function EventSheet({
  event,
  serviceName,
  onClose,
}: {
  event: WafEvent | null
  serviceName?: string
  onClose: () => void
}) {
  const qc = useQueryClient()
  const exclude = useMutation({
    mutationFn: (e: WafEvent) =>
      api.createExclusion({
        rule_id: e.rule_id,
        scope: e.service_id ? "service" : "global",
        service_id: e.service_id ?? undefined,
        path: e.path || undefined,
        target: e.matched_target || undefined,
      }),
    onSuccess: (x) => {
      qc.invalidateQueries({ queryKey: ["exclusions"] })
      qc.invalidateQueries({ queryKey: ["waf-events"] })
      qc.invalidateQueries({ queryKey: ["top-triggers"] })
      toast.success("Exclusion created", { description: `${x.rule_id} silenced on ${x.path || "this service"}` })
      onClose()
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't create the exclusion"),
  })
  const block = useMutation({
    mutationFn: (e: WafEvent) =>
      api.createBlock({ cidr: `${e.client_ip}/32`, scope: "global", mode: "block", reason: `WAF event — rule ${e.rule_id}` }),
    onSuccess: (_x, e) => {
      qc.invalidateQueries({ queryKey: ["blocklist"] })
      toast.success("IP blocked", { description: `${e.client_ip} blocked at the edge` })
      onClose()
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't block the IP"),
  })
  const busy = exclude.isPending || block.isPending

  return (
    <Sheet open={!!event} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 sm:max-w-md">
        {event && (
          <>
            <SheetHeader className="border-b">
              <div className="flex items-center gap-2">
                <SeverityBadge severity={normalizeSeverity(event.severity)} />
                <Mono>{event.rule_id}</Mono>
                {event.is_anomaly_score && (
                  <span className="rounded bg-muted px-1 font-mono text-[10px] text-muted-foreground">anomaly</span>
                )}
              </div>
              <SheetTitle className="text-left text-base font-medium leading-snug">{event.rule_msg}</SheetTitle>
            </SheetHeader>
            <div className="space-y-3 overflow-y-auto p-4 text-sm">
              <Field label="Service">{serviceName ?? "—"}</Field>
              <Field label="Request">
                <span className={methodColor(event.method)}>{event.method}</span> {event.path}
              </Field>
              <Field label="Matched target">{event.matched_target || "—"}</Field>
              {event.matched_value && <Field label="Matched value">{event.matched_value}</Field>}
              <Field label="Response">{event.status}</Field>
              <Field label="Client">{event.client_ip}</Field>
              <Field label="Engine">{event.engine_mode || "—"}</Field>
              <Field label="When">{new Date(event.ts).toLocaleString()}</Field>
            </div>
            <div className="mt-auto flex gap-2 border-t p-4">
              <Button className="flex-1" disabled={busy} onClick={() => exclude.mutate(event)} data-testid="event-create-exclusion">
                <FilterX className="size-4" /> {exclude.isPending ? "Creating…" : "Create exclusion"}
              </Button>
              <Button variant="outline" className="flex-1" disabled={busy} onClick={() => block.mutate(event)} data-testid="event-block-ip">
                <Ban className="size-4" /> {block.isPending ? "Blocking…" : "Block IP"}
              </Button>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b pb-3 last:border-0">
      <span className="shrink-0 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{label}</span>
      <span className="break-all text-right font-mono text-[13px]">{children}</span>
    </div>
  )
}
