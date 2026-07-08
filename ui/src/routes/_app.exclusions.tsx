import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Trash2 } from "lucide-react"
import { PageHeader, Mono, StatusDot, ago } from "@/components/console"
import type { Tone } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import { useServiceNames } from "@/data/queries"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

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
    </div>
  )
}
