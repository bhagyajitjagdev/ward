import { useEffect, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Gauge, Trash2, Pencil } from "lucide-react"
import { PageHeader, Mono, ago } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { RateLimit } from "@/lib/api"
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

export const Route = createFileRoute("/_app/rate-limits")({
  component: RateLimitsPage,
})

function RateLimitsPage() {
  const qc = useQueryClient()
  const names = useServiceNames()
  const { data: limits, isLoading, error } = useQuery({ queryKey: ["rate-limits"], queryFn: api.listRateLimits })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteRateLimit(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["rate-limits"] })
      toast.success("Rate limit removed")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove the rate limit"),
  })
  const [dialog, setDialog] = useState<{ open: boolean; editing: RateLimit | null }>({ open: false, editing: null })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Rate Limits"
        description="Cap requests per client IP — globally across the edge, or per service. Over the cap gets a 429."
        actions={
          <Button onClick={() => setDialog({ open: true, editing: null })}>
            <Gauge className="size-4" /> Add limit
          </Button>
        }
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Scope</th>
              <th className="px-4 py-2.5 font-medium">Limit</th>
              <th className="px-4 py-2.5 font-medium">Key</th>
              <th className="px-4 py-2.5 font-medium">Added</th>
              <th className="w-10" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading && (
              <tr>
                <td colSpan={5} className="px-4 py-3.5">
                  <Skeleton className="h-6 w-full" />
                </td>
              </tr>
            )}
            {error && (
              <tr>
                <td colSpan={5} className="py-12 text-center text-sm text-red-500">
                  Couldn't load rate limits.
                </td>
              </tr>
            )}
            {limits?.length === 0 && (
              <tr>
                <td colSpan={5} className="py-16 text-center text-sm text-muted-foreground">
                  No rate limits. Add one to cap abusive traffic before it reaches a service.
                </td>
              </tr>
            )}
            {limits?.map((rl) => (
              <tr key={rl.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3">
                  <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                    {rl.scope === "service" ? (rl.service_id ? (names[rl.service_id] ?? "service") : "service") : "global"}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <Mono className="font-medium">{rl.max_events.toLocaleString()}</Mono>
                  <Mono dim className="!text-xs">
                    {" "}
                    / {rl.window}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  <span className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">per IP</span>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(rl.created_at)}
                  </Mono>
                </td>
                <td className="pr-3">
                  <div className="flex justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-foreground"
                      aria-label="Edit rate limit"
                      onClick={() => setDialog({ open: true, editing: rl })}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-red-500"
                      aria-label="Remove rate limit"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(rl.id)}
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
      <AddDialog open={dialog.open} editing={dialog.editing} onOpenChange={(o) => setDialog((d) => ({ ...d, open: o }))} />
    </div>
  )
}

function AddDialog({ editing, open, onOpenChange }: { editing: RateLimit | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const qc = useQueryClient()
  const { data: services } = useServices()
  const [scope, setScope] = useState("global") // "global" | <service id>
  const [maxEvents, setMaxEvents] = useState("100")
  const [window, setWindow] = useState("1m")

  useEffect(() => {
    if (!open) return
    setScope(editing?.scope === "service" ? (editing.service_id ?? "global") : "global")
    setMaxEvents(editing ? String(editing.max_events) : "100")
    setWindow(editing?.window ?? "1m")
  }, [open, editing])

  const save = useMutation({
    mutationFn: () => {
      const input = {
        scope: (scope === "global" ? "global" : "service") as "global" | "service",
        service_id: scope === "global" ? undefined : scope,
        max_events: Number(maxEvents) || 0,
        window: window.trim(),
      }
      return editing ? api.updateRateLimit(editing.id, input) : api.createRateLimit(input)
    },
    onSuccess: (rl: RateLimit) => {
      qc.invalidateQueries({ queryKey: ["rate-limits"] })
      toast.success(editing ? `Updated — ${rl.max_events}/${rl.window}` : `Rate limit added — ${rl.max_events}/${rl.window} per IP`)
      onOpenChange(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save the rate limit"),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{editing ? "Edit rate limit" : "Add rate limit"}</DialogTitle>
          <DialogDescription>
            Ward applies a per-IP cap at the edge. A client over the cap gets a 429 until the window rolls.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            save.mutate()
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="scope">Scope</Label>
            <Select value={scope} onValueChange={setScope}>
              <SelectTrigger id="scope" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="global">Global · whole edge</SelectItem>
                {services?.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="max">Max requests</Label>
              <Input
                id="max"
                type="number"
                min={1}
                className="font-mono"
                value={maxEvents}
                onChange={(e) => setMaxEvents(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="window">Per window</Label>
              <Input
                id="window"
                className="font-mono"
                value={window}
                onChange={(e) => setWindow(e.target.value)}
                placeholder="1m"
              />
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Window is a duration — <Mono>10s</Mono>, <Mono>1m</Mono>, <Mono>1h</Mono>. Counted per client IP.
          </p>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending ? "Saving…" : editing ? "Save changes" : "Add rate limit"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
