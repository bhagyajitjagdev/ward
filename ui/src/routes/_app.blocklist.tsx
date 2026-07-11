import { useEffect, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Ban, Trash2, Pencil, AlertTriangle } from "lucide-react"
import { PageHeader, Mono, ago, until, ModeBadge, ModeToggle } from "@/components/console"
import type { RuleMode } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { Block } from "@/lib/api"
import { useServices, useServiceNames } from "@/data/queries"
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

function sourceStyle(source: string): string {
  if (source === "crowdsec") return "border-primary/30 bg-primary/10 text-primary"
  if (source === "threshold") return "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400"
  return "border-border bg-muted/50 text-muted-foreground"
}

export const Route = createFileRoute("/_app/blocklist")({
  component: BlocklistPage,
})

function BlocklistPage() {
  const qc = useQueryClient()
  const names = useServiceNames()
  const { data: blocks, isLoading, error } = useQuery({ queryKey: ["blocklist"], queryFn: api.listBlocklist })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteBlock(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["blocklist"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success("Unblocked")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't unblock"),
  })
  const [dialog, setDialog] = useState<{ open: boolean; editing: Block | null }>({ open: false, editing: null })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Blocklist"
        description="Block IPs and ranges at the edge — or allow only a set — globally or per service, with an optional expiry."
        actions={
          <Button onClick={() => setDialog({ open: true, editing: null })}>
            <Ban className="size-4" /> Block IP
          </Button>
        }
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">IP / CIDR</th>
              <th className="px-4 py-2.5 font-medium">Scope</th>
              <th className="px-4 py-2.5 font-medium">Reason</th>
              <th className="px-4 py-2.5 font-medium">Source</th>
              <th className="px-4 py-2.5 font-medium">Added</th>
              <th className="px-4 py-2.5 font-medium">Expires</th>
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
                  Couldn't load the blocklist.
                </td>
              </tr>
            )}
            {blocks?.length === 0 && (
              <tr>
                <td colSpan={7} className="py-16 text-center text-sm text-muted-foreground">
                  Nothing blocked. Block an address here or from a WAF event.
                </td>
              </tr>
            )}
            {blocks?.map((b) => (
              <tr key={b.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <Mono className="font-medium">{b.cidr}</Mono>
                    <ModeBadge mode={b.mode} />
                  </div>
                </td>
                <td className="px-4 py-3">
                  <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                    {b.scope === "service" ? (b.service_id ? (names[b.service_id] ?? "service") : "service") : "global"}
                  </span>
                </td>
                <td className="max-w-[280px] px-4 py-3">
                  <span className="block truncate text-muted-foreground">{b.reason || "—"}</span>
                </td>
                <td className="px-4 py-3">
                  <span
                    className={`inline-flex items-center rounded border px-1.5 py-0.5 font-mono text-[11px] ${sourceStyle(b.source)}`}
                  >
                    {b.source}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(b.created_at)}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  {b.expires_at ? (
                    <Mono dim className="!text-xs">
                      in {until(b.expires_at)}
                    </Mono>
                  ) : (
                    <span className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">never</span>
                  )}
                </td>
                <td className="pr-3">
                  <div className="flex justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-foreground"
                      aria-label="Edit rule"
                      onClick={() => setDialog({ open: true, editing: b })}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-red-500"
                      aria-label="Unblock"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(b.id)}
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
      <BlockDialog open={dialog.open} editing={dialog.editing} onOpenChange={(o) => setDialog((d) => ({ ...d, open: o }))} />
    </div>
  )
}

function BlockDialog({ editing, open, onOpenChange }: { editing: Block | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const qc = useQueryClient()
  const { data: services } = useServices()
  const [cidr, setCidr] = useState("")
  const [reason, setReason] = useState("")
  const [scope, setScope] = useState("global") // "global" | <service id>
  const [mode, setMode] = useState<RuleMode>("block")

  useEffect(() => {
    if (!open) return
    setCidr(editing?.cidr ?? "")
    setReason(editing?.reason ?? "")
    setScope(editing?.scope === "service" ? (editing.service_id ?? "global") : "global")
    setMode((editing?.mode as RuleMode) ?? "block")
  }, [open, editing])

  const save = useMutation({
    mutationFn: () => {
      const input = {
        cidr: cidr.trim(),
        reason: reason.trim() || undefined,
        mode,
        scope: (scope === "global" ? "global" : "service") as "global" | "service",
        service_id: scope === "global" ? undefined : scope,
      }
      return editing ? api.updateBlock(editing.id, input) : api.createBlock(input)
    },
    onSuccess: (b: Block) => {
      qc.invalidateQueries({ queryKey: ["blocklist"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success(editing ? `Updated ${b.cidr}` : b.mode === "allow" ? `Allowing only ${b.cidr}` : `Blocked ${b.cidr}`)
      onOpenChange(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save the rule"),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit IP rule" : "Add an IP rule"}</DialogTitle>
          <DialogDescription>
            Enter a single IP or a CIDR range. Ward reapplies the edge config immediately.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
          className="space-y-4"
        >
          <ModeToggle value={mode} onChange={setMode} />
          {mode === "allow" && (
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-600 dark:text-amber-400">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>
                Allow-only denies <strong>every</strong> IP not on the allow-list
                {scope === "global" ? " across the whole edge" : " for this service"}. Add each address that needs
                access — including your own — or you'll lock everyone out.
              </span>
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="cidr">IP or CIDR</Label>
            <Input
              id="cidr"
              className="font-mono"
              value={cidr}
              onChange={(e) => setCidr(e.target.value)}
              placeholder="185.220.101.44  or  45.155.205.0/24"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="scope">Scope</Label>
            <Select value={scope} onValueChange={setScope}>
              <SelectTrigger id="scope" className="w-full">
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
            <p className="text-xs text-muted-foreground">
              Global denies at the top of the edge; per-service denies only inside that route.
            </p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="reason">Reason</Label>
            <Input
              id="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Sustained SQLi on /api/leads/batch"
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "Applying…" : editing ? "Save changes" : mode === "allow" ? "Add to allow-list" : "Block address"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
