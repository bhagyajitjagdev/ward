import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { History, RotateCcw, FileJson, AlertTriangle } from "lucide-react"
import { PageHeader, Mono, StatusDot, ago } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { ConfigSnapshot } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export const Route = createFileRoute("/_app/snapshots")({
  component: SnapshotsPage,
})

// Every applied config is recorded here (deduplicated — the reconciler's identical
// re-applies add nothing). Rolling back restores Ward's own config (the DB) to the
// snapshot's state and re-applies it, so the reconciler keeps it: it's a real undo,
// not a temporary reload of the edge.
function SnapshotsPage() {
  const qc = useQueryClient()
  const { data: snaps, isLoading, error } = useQuery({ queryKey: ["snapshots"], queryFn: api.listSnapshots })
  const [confirm, setConfirm] = useState<ConfigSnapshot | null>(null)
  const [viewing, setViewing] = useState<ConfigSnapshot | null>(null)

  const rollback = useMutation({
    mutationFn: (id: string) => api.rollback(id),
    onSuccess: () => {
      // The whole desired state may have changed — refetch everything.
      qc.invalidateQueries()
      toast.success("Rolled back", { description: "Ward's config and the edge now match that snapshot." })
      setConfirm(null)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't roll back"),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Snapshots"
        description="Every config applied to the edge, newest first. Rolling back restores Ward's configuration to that point — services, WAF rules, IP rules, rate limits, geo, settings — and re-applies it."
        actions={
          snaps ? (
            <Mono dim className="!text-xs uppercase tracking-wider">
              {snaps.length} snapshot{snaps.length === 1 ? "" : "s"}
            </Mono>
          ) : undefined
        }
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Applied</th>
              <th className="px-4 py-2.5 font-medium">Status</th>
              <th className="px-4 py-2.5 font-medium">Note</th>
              <th className="px-4 py-2.5 font-medium">Id</th>
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
                  Couldn't load the snapshots.
                </td>
              </tr>
            )}
            {snaps?.length === 0 && (
              <tr>
                <td colSpan={5} className="py-16 text-center text-sm text-muted-foreground">
                  No config has been applied to the edge yet.
                </td>
              </tr>
            )}
            {snaps?.map((s) => (
              <tr key={s.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3">
                  <div className="flex flex-col leading-tight">
                    <span className="font-medium">{ago(s.created_at)}</span>
                    <Mono dim className="!text-[11px]">
                      {new Date(s.created_at).toLocaleString()}
                    </Mono>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-1.5">
                    {s.active ? (
                      <span className="inline-flex items-center gap-1.5 rounded border border-primary/30 bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] text-primary">
                        <StatusDot tone="armed" /> live on the edge
                      </span>
                    ) : (
                      <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                        previous
                      </span>
                    )}
                    {!s.restorable && (
                      <span
                        className="inline-flex items-center rounded border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 font-mono text-[11px] text-amber-600 dark:text-amber-400"
                        title="Taken before Ward recorded its configuration alongside the edge config — only the rendered Caddy JSON exists, so it can't be restored."
                      >
                        not restorable
                      </span>
                    )}
                  </div>
                </td>
                <td className="max-w-[320px] px-4 py-3">
                  <span className="block truncate text-muted-foreground">{s.note || "—"}</span>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {s.id.slice(0, 8)}…
                  </Mono>
                </td>
                <td className="pr-3">
                  <div className="flex justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-foreground"
                      aria-label="View the Caddy config"
                      onClick={() => setViewing(s)}
                    >
                      <FileJson className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-amber-500 disabled:opacity-30"
                      aria-label="Roll back to this snapshot"
                      disabled={s.active || !s.restorable || rollback.isPending}
                      onClick={() => setConfirm(s)}
                    >
                      <RotateCcw className="size-4" />
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <p className="flex items-start gap-2 text-xs text-muted-foreground">
        <History className="mt-0.5 size-3.5 shrink-0" />
        <span>
          Uploaded certificates, the GeoIP database and deployment settings aren't part of a snapshot — a rollback
          uses whatever is on the box now. Identical re-applies (the minute-by-minute reconcile) don't add rows.
        </span>
      </p>

      <Dialog open={!!confirm} onOpenChange={(o) => !o && setConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Roll back to this snapshot?</DialogTitle>
            <DialogDescription>
              Ward's configuration will be restored to how it was{" "}
              {confirm ? new Date(confirm.created_at).toLocaleString() : ""} and re-applied to the edge. Every
              change made since — services, WAF exclusions and custom rules, IP rules, rate limits, geo rules,
              trusted IPs and edge settings — is undone. This is recorded in the audit log.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-600 dark:text-amber-400">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            <span>
              Users, API tokens, detections, the access log and retention settings are not touched. The rollback
              itself becomes a new snapshot, so you can roll forward again from here.
            </span>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirm(null)}>
              Cancel
            </Button>
            <Button disabled={rollback.isPending} onClick={() => confirm && rollback.mutate(confirm.id)}>
              <RotateCcw className="size-4" /> {rollback.isPending ? "Rolling back…" : "Roll back"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <SnapshotDialog snapshot={viewing} onClose={() => setViewing(null)} />
    </div>
  )
}

// SnapshotDialog shows the rendered Caddy JSON a snapshot pushed — for eyeballing
// what changed, or copying into a diff.
function SnapshotDialog({ snapshot, onClose }: { snapshot: ConfigSnapshot | null; onClose: () => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ["snapshot", snapshot?.id],
    queryFn: () => api.getSnapshot(snapshot!.id),
    enabled: !!snapshot,
  })
  return (
    <Dialog open={!!snapshot} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Caddy config</DialogTitle>
          <DialogDescription>
            {snapshot ? `Applied ${new Date(snapshot.created_at).toLocaleString()}` : ""}
            {snapshot?.note ? ` · ${snapshot.note}` : ""}
          </DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : (
          <pre className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 font-mono text-[11px] leading-relaxed">
            {data?.caddy_json ?? ""}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  )
}
