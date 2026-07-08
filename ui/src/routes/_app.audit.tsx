import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { PageHeader, Mono, ago } from "@/components/console"
import { api } from "@/lib/api"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Skeleton } from "@/components/ui/skeleton"

export const Route = createFileRoute("/_app/audit")({
  component: AuditPage,
})

function actionStyle(action: string): string {
  const verb = action.split(".").pop() ?? ""
  if (verb === "create") return "border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
  if (["block", "delete", "revoke"].includes(verb)) return "border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400"
  if (verb === "rollback") return "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (verb === "update") return "border-primary/30 bg-primary/10 text-primary"
  return "border-border bg-muted/50 text-muted-foreground"
}

function AuditPage() {
  const { data: entries, isLoading, error } = useQuery({
    queryKey: ["audit-log"],
    queryFn: () => api.listAuditLog(200),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Audit Log"
        description="Every change to the edge — who did it, to what, and when. Append-only."
        actions={
          entries ? (
            <Mono dim className="!text-xs uppercase tracking-wider">
              {entries.length} entries
            </Mono>
          ) : undefined
        }
      />

      <div className="overflow-hidden rounded-xl border bg-card">
        {isLoading ? (
          <div className="space-y-3 p-5">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-6 w-2/3" />
          </div>
        ) : error ? (
          <div className="py-12 text-center text-sm text-red-500">Couldn't load the audit log.</div>
        ) : entries?.length === 0 ? (
          <div className="py-16 text-center text-sm text-muted-foreground">Nothing's happened yet.</div>
        ) : (
          <ol className="divide-y">
            {entries?.map((e) => (
              <li key={e.id} className="flex items-center gap-4 px-5 py-3.5">
                <Avatar className="size-8 shrink-0 rounded-md">
                  <AvatarFallback className="rounded-md bg-muted text-xs font-medium">
                    {e.actor.slice(0, 1).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="text-sm font-medium">{e.actor}</span>
                  <span
                    className={`inline-flex items-center rounded border px-1.5 py-0.5 font-mono text-[11px] ${actionStyle(e.action)}`}
                  >
                    {e.action}
                  </span>
                  {e.target && (
                    <Mono dim className="truncate !text-xs">
                      {e.target}
                    </Mono>
                  )}
                </div>
                <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{ago(e.created_at)}</span>
              </li>
            ))}
          </ol>
        )}
      </div>
    </div>
  )
}
