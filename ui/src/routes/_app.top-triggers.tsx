import { useEffect, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { FilterX, Users } from "lucide-react"
import { PageHeader, SeverityBadge, Mono, ago, normalizeSeverity } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { WafTrigger } from "@/lib/api"
import { useServiceNames } from "@/data/queries"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export const Route = createFileRoute("/_app/top-triggers")({
  component: TopTriggersPage,
})

function TopTriggersPage() {
  const [tuning, setTuning] = useState<WafTrigger | null>(null)
  const names = useServiceNames()
  const { data: triggers, isLoading, error } = useQuery({
    queryKey: ["top-triggers"],
    queryFn: () => api.topTriggers({ limit: 20 }),
    refetchInterval: 5000, // live monitor
  })

  const max = triggers?.[0]?.hits ?? 1
  const serviceLabel = (t: WafTrigger) => (t.service_id ? (names[t.service_id] ?? "service") : t.host || "global")

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Monitor"
        title="Top Triggers"
        description="The rules firing most, clustered by service, path, and matched field. Tune the loudest false positives first — Ward writes the scoped exclusion for you."
        actions={<Mono dim className="!text-xs uppercase tracking-wider">last 24h</Mono>}
      />

      {isLoading && (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-24 w-full rounded-xl" />
          ))}
        </div>
      )}
      {error && <div className="py-12 text-center text-sm text-red-500">Couldn't load top triggers.</div>}
      {triggers?.length === 0 && (
        <div className="flex min-h-[240px] flex-col items-center justify-center rounded-xl border border-dashed bg-card/40 text-center">
          <p className="text-sm font-medium">Nothing's tripping the WAF</p>
          <p className="mt-1 max-w-sm text-sm text-muted-foreground">
            When rules start firing, the loudest clusters show up here — ready to tune.
          </p>
        </div>
      )}

      <div className="space-y-3">
        {triggers?.map((t, i) => (
          <div
            key={`${t.rule_id}-${t.path}-${t.matched_target ?? ""}`}
            className="group grid grid-cols-1 items-center gap-4 rounded-xl border bg-card p-4 md:grid-cols-[1fr_auto]"
          >
            <div className="min-w-0 space-y-2">
              <div className="flex items-center gap-2.5">
                <span className="font-mono text-xs text-muted-foreground">#{i + 1}</span>
                <SeverityBadge severity={normalizeSeverity(t.severity)} />
                <Mono className="font-medium">{t.rule_id}</Mono>
                <span className="truncate text-sm text-muted-foreground">{t.rule_msg}</span>
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                <Chip>{serviceLabel(t)}</Chip>
                <Chip>{t.path}</Chip>
                {t.matched_target && <Chip accent>{t.matched_target}</Chip>}
              </div>
              <div className="flex items-center gap-3">
                <div className="h-1.5 w-full max-w-md overflow-hidden rounded-full bg-muted">
                  <div className="h-full rounded-full bg-primary/80" style={{ width: `${(t.hits / max) * 100}%` }} />
                </div>
              </div>
            </div>

            <div className="flex items-center gap-5 md:gap-6">
              <Stat value={t.hits.toLocaleString()} label="hits" />
              <Stat value={String(t.distinct_ips)} label="IPs" icon />
              <div className="hidden text-right lg:block">
                <div className="font-mono text-sm tabular-nums">{ago(t.last_seen)}</div>
                <div className="font-mono text-[10px] uppercase tracking-wide text-muted-foreground">last seen</div>
              </div>
              <Button variant="outline" onClick={() => setTuning(t)}>
                <FilterX className="size-4" /> Create exclusion
              </Button>
            </div>
          </div>
        ))}
      </div>

      <ExclusionDialog trigger={tuning} serviceName={tuning?.service_id ? names[tuning.service_id] : undefined} onClose={() => setTuning(null)} />
    </div>
  )
}

function Chip({ children, accent }: { children: React.ReactNode; accent?: boolean }) {
  return (
    <span
      className={
        accent
          ? "rounded border border-primary/30 bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] text-primary"
          : "rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
      }
    >
      {children}
    </span>
  )
}

function Stat({ value, label, icon }: { value: string; label: string; icon?: boolean }) {
  return (
    <div className="text-right">
      <div className="flex items-center justify-end gap-1 font-mono text-lg font-medium tabular-nums">
        {icon && <Users className="size-3.5 text-muted-foreground" />}
        {value}
      </div>
      <div className="font-mono text-[10px] uppercase tracking-wide text-muted-foreground">{label}</div>
    </div>
  )
}

function ExclusionDialog({
  trigger,
  serviceName,
  onClose,
}: {
  trigger: WafTrigger | null
  serviceName?: string
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [path, setPath] = useState("")
  const [target, setTarget] = useState("")
  // sync the editable fields whenever a different trigger is opened
  useEffect(() => {
    setPath(trigger?.path ?? "")
    setTarget(trigger?.matched_target ?? "")
  }, [trigger])

  const create = useMutation({
    mutationFn: (t: WafTrigger) =>
      api.createExclusion({
        rule_id: t.rule_id,
        scope: t.service_id ? "service" : "global",
        service_id: t.service_id ?? undefined,
        path: path.trim() || undefined,
        target: target.trim() || undefined,
      }),
    onSuccess: (x) => {
      qc.invalidateQueries({ queryKey: ["exclusions"] })
      qc.invalidateQueries({ queryKey: ["top-triggers"] })
      qc.invalidateQueries({ queryKey: ["waf-events"] })
      toast.success("Exclusion created", { description: `${x.rule_id} silenced on ${x.path || "this service"}` })
      onClose()
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't create the exclusion"),
  })

  const preview = trigger
    ? `SecRule REQUEST_URI "@beginsWith ${path || "/"}" \\
  "id:‹auto›,phase:1,pass,nolog,\\
   ctl:ruleRemoveTargetById=${trigger.rule_id};${target || "ARGS"}"`
    : ""

  return (
    <Dialog open={!!trigger} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        {trigger && (
          <>
            <DialogHeader>
              <DialogTitle>Create scoped exclusion</DialogTitle>
              <DialogDescription>
                Tell the WAF to stop inspecting <Mono className="break-all">{target || "this field"}</Mono> for rule{" "}
                <Mono>{trigger.rule_id}</Mono> — only on this path{trigger.service_id ? ", only for this service" : ""}.
                Everywhere else it stays armed.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-3 text-sm">
              <div className="grid grid-cols-[100px_1fr] items-center gap-x-4 gap-y-2.5">
                <Label>Scope</Label>
                <Mono>{trigger.service_id ? (serviceName ?? "service") : "global"}</Mono>
                <Label>Rule</Label>
                <Mono>{trigger.rule_id}</Mono>
                <Label>Path</Label>
                <Input className="h-8 font-mono text-xs" value={path} onChange={(e) => setPath(e.target.value)} placeholder="/" />
                <Label>Field</Label>
                <Input className="h-8 break-all font-mono text-xs" value={target} onChange={(e) => setTarget(e.target.value)} placeholder="whole rule (leave blank)" />
              </div>

              <div>
                <Label>Generated rule</Label>
                <pre className="mt-1.5 overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed text-muted-foreground">
                  {preview}
                </pre>
              </div>
            </div>

            <DialogFooter>
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button onClick={() => create.mutate(trigger)} disabled={create.isPending}>
                {create.isPending ? "Creating…" : "Create exclusion"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Label({ children }: { children: React.ReactNode }) {
  return <span className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{children}</span>
}
