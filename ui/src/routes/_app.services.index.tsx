import { useState } from "react"
import { createFileRoute, useNavigate } from "@tanstack/react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Plus, ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"
import { PageHeader, StatusDot, Mono } from "@/components/console"
import { useServices } from "@/data/queries"
import { api, ApiError } from "@/lib/api"
import type { Service } from "@/lib/api"
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

export const Route = createFileRoute("/_app/services/")({
  component: ServicesPage,
})

function ServicesPage() {
  const { data: services, isLoading, error } = useServices()
  const navigate = useNavigate()

  return (
    <div className="space-y-8">
      <PageHeader
        eyebrow="Edge"
        title="Services"
        description="Backends Ward reverse-proxies and protects. Each becomes a Caddy route with its own WAF, TLS, and load-balancing."
        actions={<CreateServiceDialog />}
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Service</th>
              <th className="px-4 py-2.5 font-medium">Public hostname</th>
              <th className="px-4 py-2.5 font-medium">WAF</th>
              <th className="px-4 py-2.5 font-medium">TLS</th>
              <th className="px-4 py-2.5 font-medium">Status</th>
              <th className="w-8" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading &&
              [0, 1, 2].map((i) => (
                <tr key={i}>
                  <td colSpan={6} className="px-4 py-3.5">
                    <Skeleton className="h-6 w-full" />
                  </td>
                </tr>
              ))}
            {error && (
              <tr>
                <td colSpan={6} className="py-12 text-center text-sm text-red-500">
                  Couldn't load services. Is the backend running?
                </td>
              </tr>
            )}
            {services?.length === 0 && (
              <tr>
                <td colSpan={6} className="py-16 text-center text-sm text-muted-foreground">
                  No services yet — add your first one to put it behind the edge.
                </td>
              </tr>
            )}
            {services?.map((s) => (
              <tr
                key={s.id}
                onClick={() => navigate({ to: "/services/$id", params: { id: s.id } })}
                className="group cursor-pointer transition-colors hover:bg-muted/40"
              >
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2.5">
                    <StatusDot tone={s.enabled ? (s.waf_enabled ? "armed" : "ok") : "idle"} />
                    <div className="min-w-0">
                      <div className="font-medium">{s.name}</div>
                      <Mono dim className="block truncate !text-xs">
                        {s.upstreams.join(", ")} · {s.lb_policy}
                      </Mono>
                    </div>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <Mono>{s.public_hostname}</Mono>
                </td>
                <td className="px-4 py-3">
                  <WafBadge on={s.waf_enabled} />
                </td>
                <td className="px-4 py-3">
                  <TlsBadge mode={s.tls_mode} />
                </td>
                <td className="px-4 py-3">
                  <span className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">
                    {s.enabled ? "active" : "disabled"}
                  </span>
                </td>
                <td className="pr-3">
                  <ChevronRight className="size-4 text-muted-foreground/40 transition-colors group-hover:text-muted-foreground" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function WafBadge({ on }: { on: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-[11px]",
        on ? "border-primary/30 bg-primary/10 text-primary" : "border-border bg-muted/50 text-muted-foreground",
      )}
    >
      <StatusDot tone={on ? "armed" : "idle"} /> {on ? "armed" : "off"}
    </span>
  )
}

function TlsBadge({ mode }: { mode: Service["tls_mode"] }) {
  const label = mode === "managed" ? "acme" : mode || "none"
  return (
    <span className="inline-flex items-center rounded border bg-muted/40 px-2 py-0.5 font-mono text-[11px] text-muted-foreground">
      {label}
    </span>
  )
}

function CreateServiceDialog() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [hostname, setHostname] = useState("")
  const [upstreams, setUpstreams] = useState("")
  const [tlsMode, setTlsMode] = useState("managed")
  const [lbPolicy, setLbPolicy] = useState("round_robin")
  const [wafEnabled, setWafEnabled] = useState(true)

  const create = useMutation({
    mutationFn: () =>
      api.createService({
        name: name.trim(),
        public_hostname: hostname.trim(),
        upstreams: upstreams
          .split(",")
          .map((u) => u.trim())
          .filter(Boolean),
        tls_mode: tlsMode,
        lb_policy: lbPolicy,
        waf_enabled: wafEnabled,
      }),
    onSuccess: (svc) => {
      qc.invalidateQueries({ queryKey: ["services"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success(`Service “${svc.name}” created`, { description: "Route generated and applied to the edge." })
      setOpen(false)
      setName("")
      setHostname("")
      setUpstreams("")
      setTlsMode("managed")
      setLbPolicy("round_robin")
      setWafEnabled(true)
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? err.message : "Couldn't create the service"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="size-4" /> Add service
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add service</DialogTitle>
          <DialogDescription>
            Ward generates the Caddy route, validates it, and applies it to the edge — with a snapshot you can roll back to.
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
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="app-api" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="hostname">Public hostname</Label>
            <Input
              id="hostname"
              className="font-mono"
              value={hostname}
              onChange={(e) => setHostname(e.target.value)}
              placeholder="api.acme.com"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="upstreams">
              Upstreams <span className="font-normal text-muted-foreground">— comma-separated host:port</span>
            </Label>
            <Input
              id="upstreams"
              className="font-mono"
              value={upstreams}
              onChange={(e) => setUpstreams(e.target.value)}
              placeholder="api-1.mesh:8000, api-2.mesh:8000"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="tls">TLS</Label>
              <Select value={tlsMode} onValueChange={setTlsMode}>
                <SelectTrigger id="tls" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="managed">Managed · Let's Encrypt</SelectItem>
                  <SelectItem value="internal">Internal CA · self-signed</SelectItem>
                  <SelectItem value="none">None · HTTP only</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="lb">Load balancing</Label>
              <Select value={lbPolicy} onValueChange={setLbPolicy}>
                <SelectTrigger id="lb" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="round_robin">Round robin</SelectItem>
                  <SelectItem value="least_conn">Least connections</SelectItem>
                  <SelectItem value="random">Random</SelectItem>
                  <SelectItem value="ip_hash">IP hash · sticky</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <label className="flex items-center gap-2.5 rounded-lg border bg-muted/30 p-3 text-sm">
            <input
              type="checkbox"
              checked={wafEnabled}
              onChange={(e) => setWafEnabled(e.target.checked)}
              className="size-4 accent-primary"
            />
            <span>
              <span className="font-medium">Protect with the WAF</span>
              <span className="block text-xs text-muted-foreground">
                Coraza + OWASP CRS in DetectionOnly — logs, doesn't block, until you flip it.
              </span>
            </span>
          </label>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Creating…" : "Create service"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
