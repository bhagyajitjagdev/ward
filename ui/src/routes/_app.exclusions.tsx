import { useEffect, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Trash2, Pencil, Plus, Power, AlertTriangle } from "lucide-react"
import { PageHeader, Mono, StatusDot, ago } from "@/components/console"
import type { Tone } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { WafCustomRule } from "@/lib/api"
import { useServiceNames, useServices } from "@/data/queries"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

function stateTone(state: string): Tone {
  if (state === "soaking") return "detecting"
  if (state === "draft") return "idle"
  return "ok" // active
}

export const Route = createFileRoute("/_app/exclusions")({
  component: ExclusionsPage,
})

function ExclusionsPage() {
  const qc = useQueryClient()
  const names = useServiceNames()
  const { data: exclusions, isLoading, error } = useQuery({
    queryKey: ["exclusions"],
    queryFn: api.listExclusions,
  })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteExclusion(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["exclusions"] })
      toast.success("Exclusion removed — the WAF is armed again there")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove exclusion"),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Exclusions"
        description="Scoped rules that tell the WAF to stand down on one path or field — without weakening it anywhere else. Create these from Top Triggers or a WAF event."
        actions={
          exclusions ? (
            <Mono dim className="!text-xs uppercase tracking-wider">
              {exclusions.length} active
            </Mono>
          ) : undefined
        }
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Rule</th>
              <th className="px-4 py-2.5 font-medium">Scope</th>
              <th className="px-4 py-2.5 font-medium">Path</th>
              <th className="px-4 py-2.5 font-medium">Field</th>
              <th className="px-4 py-2.5 font-medium">State</th>
              <th className="px-4 py-2.5 font-medium">Created</th>
              <th className="w-10" />
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
                  Couldn't load exclusions.
                </td>
              </tr>
            )}
            {exclusions?.length === 0 && (
              <tr>
                <td colSpan={7} className="py-16 text-center text-sm text-muted-foreground">
                  No exclusions yet. When the WAF flags a legitimate request, open it from Top Triggers and create a
                  scoped exclusion.
                </td>
              </tr>
            )}
            {exclusions?.map((x) => (
              <tr key={x.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3">
                  <Mono className="font-medium">{x.rule_id}</Mono>
                </td>
                <td className="px-4 py-3">
                  <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                    {x.scope === "service" ? (x.service_id ? (names[x.service_id] ?? "service") : "service") : "global"}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <Mono>{x.path || "—"}</Mono>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {x.target || "—"}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  <span className="inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wide">
                    <StatusDot tone={stateTone(x.state)} pulse={x.state === "soaking"} />
                    {x.state}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(x.created_at)}
                  </Mono>
                </td>
                <td className="pr-3">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                    aria-label="Remove exclusion"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(x.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <CustomRulesSection />
    </div>
  )
}

// CustomRulesSection is the advanced escape hatch: user-authored raw SecLang,
// injected before the CRS rules (after Ward's generated exclusions). Every save
// is validated by pushing the config to the edge — a rejected rule never sticks.
function CustomRulesSection() {
  const qc = useQueryClient()
  const names = useServiceNames()
  const { data: rules, isLoading } = useQuery({ queryKey: ["waf-custom-rules"], queryFn: api.listWafCustomRules })
  const [dialog, setDialog] = useState<{ open: boolean; editing: WafCustomRule | null }>({
    open: false,
    editing: null,
  })

  const invalidate = () => qc.invalidateQueries({ queryKey: ["waf-custom-rules"] })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteWafCustomRule(id),
    onSuccess: () => {
      invalidate()
      toast.success("Custom rule removed")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove the rule"),
  })
  const toggle = useMutation({
    mutationFn: (r: WafCustomRule) =>
      api.updateWafCustomRule(r.id, {
        name: r.name,
        seclang: r.seclang,
        scope: r.scope,
        service_id: r.service_id,
        enabled: !r.enabled,
      }),
    onSuccess: (r: WafCustomRule) => {
      invalidate()
      toast.success(r.enabled ? `${r.name} enabled` : `${r.name} disabled`)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't update the rule"),
  })

  return (
    <div className="space-y-3 pt-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h2 className="font-heading text-sm font-semibold">
            Custom rules <span className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">· advanced</span>
          </h2>
          <p className="mt-1 max-w-2xl text-xs text-muted-foreground">
            Raw SecLang, for what the exclusion builder can't express — your own SecRules, regex conditions,
            per-method logic. Rules run before the OWASP CRS. Each save is validated against the edge first.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={() => setDialog({ open: true, editing: null })}>
          <Plus className="size-4" /> New rule
        </Button>
      </div>

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Name</th>
              <th className="px-4 py-2.5 font-medium">Scope</th>
              <th className="px-4 py-2.5 font-medium">SecLang</th>
              <th className="px-4 py-2.5 font-medium">Status</th>
              <th className="px-4 py-2.5 font-medium">Updated</th>
              <th className="w-24" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading && (
              <tr>
                <td colSpan={6} className="px-4 py-3.5">
                  <Skeleton className="h-6 w-full" />
                </td>
              </tr>
            )}
            {rules?.length === 0 && (
              <tr>
                <td colSpan={6} className="py-10 text-center text-sm text-muted-foreground">
                  No custom rules. Most tuning belongs in scoped exclusions — reach for raw SecLang when the
                  builder can't express what you need.
                </td>
              </tr>
            )}
            {rules?.map((r) => (
              <tr key={r.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3 font-medium">{r.name}</td>
                <td className="px-4 py-3">
                  <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                    {r.scope === "service" ? (r.service_id ? (names[r.service_id] ?? "service") : "service") : "global"}
                  </span>
                </td>
                <td className="max-w-[26rem] px-4 py-3">
                  <Mono dim className="line-clamp-1 !text-xs">
                    {r.seclang}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  <span className="inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wide">
                    <StatusDot tone={r.enabled ? "ok" : "idle"} />
                    {r.enabled ? "enabled" : "disabled"}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(r.updated_at)}
                  </Mono>
                </td>
                <td className="pr-3">
                  <div className="flex items-center justify-end gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground"
                      aria-label={r.enabled ? "Disable rule" : "Enable rule"}
                      disabled={toggle.isPending}
                      onClick={() => toggle.mutate(r)}
                    >
                      <Power className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground"
                      aria-label="Edit rule"
                      onClick={() => setDialog({ open: true, editing: r })}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-red-500"
                      aria-label="Remove rule"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(r.id)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <CustomRuleDialog
        open={dialog.open}
        editing={dialog.editing}
        onOpenChange={(o) => setDialog((d) => ({ ...d, open: o }))}
      />
    </div>
  )
}

function CustomRuleDialog({
  editing,
  open,
  onOpenChange,
}: {
  editing: WafCustomRule | null
  open: boolean
  onOpenChange: (o: boolean) => void
}) {
  const qc = useQueryClient()
  const { data: services } = useServices()
  const [name, setName] = useState("")
  const [scope, setScope] = useState("global") // "global" | <service id>
  const [seclang, setSeclang] = useState("")

  useEffect(() => {
    if (!open) return
    setName(editing?.name ?? "")
    setScope(editing?.scope === "service" ? (editing.service_id ?? "global") : "global")
    setSeclang(editing?.seclang ?? "")
  }, [open, editing])

  const save = useMutation({
    mutationFn: () => {
      const input = {
        name: name.trim(),
        seclang: seclang.trim(),
        scope: (scope === "global" ? "global" : "service") as "global" | "service",
        service_id: scope === "global" ? undefined : scope,
        enabled: editing?.enabled ?? true,
      }
      return editing ? api.updateWafCustomRule(editing.id, input) : api.createWafCustomRule(input)
    },
    onSuccess: (r: WafCustomRule) => {
      qc.invalidateQueries({ queryKey: ["waf-custom-rules"] })
      toast.success(editing ? `Updated ${r.name}` : `${r.name} is live on the edge`)
      onOpenChange(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save the rule"),
  })

  const ready = name.trim() && seclang.trim()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{editing ? "Edit custom rule" : "New custom rule"}</DialogTitle>
          <DialogDescription>
            Raw SecLang, injected before the OWASP CRS (after Ward's generated exclusions). Ward pushes the
            config to the edge before keeping it — invalid SecLang is rejected with the edge's error.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (ready) save.mutate()
          }}
          className="space-y-4"
        >
          <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-600 dark:text-amber-400">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            <span>
              This is your WAF's escape hatch — a rule here can weaken protection (e.g.{" "}
              <Mono className="!text-[11px]">SecRuleRemoveById</Mono>) as easily as strengthen it. Prefer scoped
              exclusions for tuning; keep rule ids in the 1–99,999 range to avoid CRS collisions.
            </span>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="rule-name">Name</Label>
              <Input
                id="rule-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Deny TRACE everywhere"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="rule-scope">Scope</Label>
              <Select value={scope} onValueChange={setScope}>
                <SelectTrigger id="rule-scope" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="global">Global · every service</SelectItem>
                  {services?.map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="rule-seclang">SecLang</Label>
            <textarea
              id="rule-seclang"
              value={seclang}
              onChange={(e) => setSeclang(e.target.value)}
              rows={7}
              spellCheck={false}
              placeholder={'SecRule REQUEST_METHOD "@streq TRACE" \\\n    "id:1001,phase:1,deny,status:405,msg:\'TRACE blocked\'"'}
              className="w-full resize-y rounded-md border bg-background px-3 py-2 font-mono text-xs leading-relaxed shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
            <p className="text-xs text-muted-foreground">
              Multi-line SecLang is fine — chains, comments, several rules. It lands verbatim in this scope's WAF
              directives.
            </p>
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!ready || save.isPending}>
              {save.isPending ? "Validating on the edge…" : editing ? "Save changes" : "Validate & apply"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
