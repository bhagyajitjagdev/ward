import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Ban, Trash2 } from "lucide-react"
import { PageHeader, Mono, ago, until } from "@/components/console"
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
  DialogTrigger,
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

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Blocklist"
        description="IPs and ranges denied at the edge before they reach a service — globally or per service, with an optional expiry."
        actions={<BlockDialog />}
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
                  <Mono className="font-medium">{b.cidr}</Mono>
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
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                    aria-label="Unblock"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(b.id)}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function BlockDialog() {
  const qc = useQueryClient()
  const { data: services } = useServices()
  const [open, setOpen] = useState(false)
  const [cidr, setCidr] = useState("")
  const [reason, setReason] = useState("")
  const [scope, setScope] = useState("global") // "global" | <service id>

  const create = useMutation({
    mutationFn: () =>
      api.createBlock({
        cidr: cidr.trim(),
        reason: reason.trim() || undefined,
        scope: scope === "global" ? "global" : "service",
        service_id: scope === "global" ? undefined : scope,
      }),
    onSuccess: (b: Block) => {
      qc.invalidateQueries({ queryKey: ["blocklist"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success(`Blocked ${b.cidr}`)
      setOpen(false)
      setCidr("")
      setReason("")
      setScope("global")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't block the address"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Ban className="size-4" /> Block IP
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Block an address</DialogTitle>
          <DialogDescription>
            Ward denies it at the edge with a 403 and reapplies the config. Enter a single IP or a CIDR range.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate()
          }}
          className="space-y-4"
        >
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
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Blocking…" : "Block address"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
