import { useState } from "react"
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { ArrowLeft, Pencil, Trash2 } from "lucide-react"
import { cn } from "@/lib/utils"
import { certForHost } from "@/lib/certs"
import { PageHeader, StatusDot, SeverityBadge, Mono, ago, normalizeSeverity } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { Service, WafMode } from "@/lib/api"
import {
  ServiceFormFields,
  serviceToForm,
  formToInput,
  serviceFormValid,
  type ServiceFormState,
} from "@/components/service-form"
import { Button } from "@/components/ui/button"
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

export const Route = createFileRoute("/_app/services/$id")({
  component: ServiceDetailPage,
})

function ServiceDetailPage() {
  const { id } = Route.useParams()
  const { data: svc, isLoading, error } = useQuery({ queryKey: ["service", id], queryFn: () => api.getService(id) })
  const { data: events } = useQuery({
    queryKey: ["waf-events", "service", id],
    queryFn: () => api.listWafEvents({ service_id: id, limit: 8 }),
  })
  const { data: allExclusions } = useQuery({ queryKey: ["exclusions"], queryFn: api.listExclusions })
  const { data: allBlocks } = useQuery({ queryKey: ["blocklist"], queryFn: api.listBlocklist })
  const { data: settings } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const { data: certificates } = useQuery({ queryKey: ["certificates"], queryFn: api.listCertificates })
  const exclusions = (allExclusions ?? []).filter((x) => x.service_id === id)
  const blocks = (allBlocks ?? []).filter((b) => b.service_id === id)

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-40 w-full rounded-xl" />
      </div>
    )
  }
  if (error || !svc) {
    return (
      <div className="space-y-4">
        <BackLink />
        <div className="rounded-xl border border-dashed py-16 text-center text-sm text-muted-foreground">
          This service doesn't exist (or was deleted).
        </div>
      </div>
    )
  }

  const effectiveWafMode: WafMode = svc.waf_mode || settings?.waf_engine_mode || "DetectionOnly"
  // Match by SAN, not the storage-folder name — a cert named sv1 also serves sv2 if sv2 is in its SAN.
  // A custom cert must cover every hostname (backend enforces); show the one that does.
  const customCert =
    svc.tls_mode === "custom"
      ? svc.public_hostnames.map((h) => certForHost(certificates, h)).find(Boolean)
      : undefined

  return (
    <div className="space-y-8">
      <div className="space-y-4">
        <BackLink />
        <PageHeader
          eyebrow="Service"
          title={svc.name}
          description={svc.public_hostname}
          actions={
            <>
              <EditDialog service={svc} />
              <DeleteDialog service={svc} />
            </>
          }
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <section className="rounded-xl border bg-card lg:col-span-2">
          <PanelHead title="Configuration" />
          <dl className="grid grid-cols-1 gap-x-8 gap-y-4 p-5 sm:grid-cols-2">
            <Field label="Status">
              <span className="inline-flex items-center gap-1.5">
                <StatusDot tone={svc.enabled ? "ok" : "idle"} /> {svc.enabled ? "active" : "disabled"}
              </span>
            </Field>
            <Field label="WAF">
              {svc.waf_enabled ? (
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot tone={effectiveWafMode === "On" ? "threat" : "detecting"} />
                  {effectiveWafMode === "On" ? "enforcing" : "detection-only"}
                  <span className="font-mono text-[11px] text-muted-foreground">
                    {svc.waf_mode ? "· override" : "· inherited"}
                  </span>
                </span>
              ) : (
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot tone="idle" /> off
                </span>
              )}
            </Field>
            <Field label={svc.public_hostnames.length > 1 ? "Public hostnames" : "Public hostname"} mono>
              {svc.public_hostnames.length > 1 ? (
                <div className="flex flex-wrap gap-1">
                  {svc.public_hostnames.map((h) => (
                    <span key={h} className="rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px]">
                      {h}
                    </span>
                  ))}
                </div>
              ) : (
                svc.public_hostname
              )}
            </Field>
            <Field label="TLS">
              {svc.tls_mode === "custom" ? (
                <span className="inline-flex items-center gap-1.5">
                  <StatusDot tone={customCert ? "ok" : "detecting"} />
                  <span className="font-mono !text-[13px]">custom</span>
                  {customCert ? (
                    <span className="font-mono text-[11px] text-muted-foreground">
                      · expires {new Date(customCert.not_after).toISOString().slice(0, 10)}
                    </span>
                  ) : (
                    <Link to="/certificates" className="font-mono text-[11px] text-amber-500 hover:underline">
                      · no cert uploaded
                    </Link>
                  )}
                </span>
              ) : (
                <span className="font-mono !text-[13px]">{svc.tls_mode === "managed" ? "acme" : svc.tls_mode}</span>
              )}
            </Field>
            <Field label="Load balancing" mono>
              {svc.lb_policy}
            </Field>
            <Field label="Created" mono>
              {ago(svc.created_at)}
            </Field>
            <div className="sm:col-span-2">
              <Field label="Upstreams" mono>
                {svc.upstreams.join(", ")}
              </Field>
            </div>
          </dl>
        </section>

        <section className="rounded-xl border bg-card">
          <PanelHead title="Protection" />
          <div className="divide-y">
            <CountRow label="Exclusions" value={exclusions.length} to={`/exclusions`} />
            <CountRow label="IP blocks" value={blocks.length} to={`/blocklist`} />
            <CountRow label="Detections shown" value={events?.length ?? 0} to={`/waf-events`} />
          </div>
        </section>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <section className="rounded-xl border bg-card">
          <PanelHead title="Recent detections" hint="this service" />
          <div className="p-5">
            {events && events.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">No detections for this service.</p>
            ) : (
              <ul className="-my-1 divide-y">
                {(events ?? []).map((e) => (
                  <li key={e.id} className="flex items-center gap-2.5 py-2.5 text-sm">
                    <SeverityBadge severity={normalizeSeverity(e.severity)} />
                    <Mono className="shrink-0">{e.rule_id}</Mono>
                    <Mono dim className="min-w-0 flex-1 truncate !text-xs">
                      {e.method} {e.path}
                    </Mono>
                    <span className="shrink-0 font-mono text-[11px] text-muted-foreground">{ago(e.ts)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>

        <section className="rounded-xl border bg-card">
          <PanelHead title="Exclusions" hint="this service" />
          <div className="p-5">
            {exclusions.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">No scoped exclusions here.</p>
            ) : (
              <ul className="-my-1 divide-y">
                {exclusions.map((x) => (
                  <li key={x.id} className="flex items-center gap-2.5 py-2.5 text-sm">
                    <Mono className="shrink-0 font-medium">{x.rule_id}</Mono>
                    <Mono dim className="min-w-0 flex-1 truncate !text-xs">
                      {x.path || "—"} · {x.target || "—"}
                    </Mono>
                    <span className="shrink-0 font-mono text-[11px] uppercase tracking-wide text-muted-foreground">
                      {x.state}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      </div>
    </div>
  )
}

function BackLink() {
  return (
    <Link
      to="/services"
      className="inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="size-3.5" /> Services
    </Link>
  )
}

function PanelHead({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="flex items-baseline gap-2 border-b px-5 py-3.5">
      <h2 className="font-heading text-sm font-semibold">{title}</h2>
      {hint && <span className="font-mono text-[11px] text-muted-foreground">{hint}</span>}
    </div>
  )
}

function Field({ label, children, mono }: { label: string; children: React.ReactNode; mono?: boolean }) {
  return (
    <div>
      <dt className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{label}</dt>
      <dd className={cn("mt-1 text-sm", mono && "font-mono !text-[13px]")}>{children}</dd>
    </div>
  )
}

function CountRow({ label, value, to }: { label: string; value: number; to: "/exclusions" | "/blocklist" | "/waf-events" }) {
  return (
    <Link to={to} className="flex items-center justify-between px-5 py-3 text-sm transition-colors hover:bg-muted/40">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono tabular-nums">{value}</span>
    </Link>
  )
}

function EditDialog({ service }: { service: Service }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<ServiceFormState>(() => serviceToForm(service))

  // Reset the form to the service's current values whenever the dialog opens, so a
  // cancelled edit doesn't leave stale local state behind on the next open.
  function onOpenChange(next: boolean) {
    if (next) setForm(serviceToForm(service))
    setOpen(next)
  }

  const save = useMutation({
    mutationFn: () => api.updateService(service.id, formToInput(form)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["service", service.id] })
      qc.invalidateQueries({ queryKey: ["services"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success("Service updated", { description: "Config regenerated and applied to the edge." })
      setOpen(false)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't update the service"),
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>
        <Button variant="outline">
          <Pencil className="size-4" /> Edit
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Edit service</DialogTitle>
          <DialogDescription>Changes regenerate the Caddy route and apply to the edge, with a rollback snapshot.</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (serviceFormValid(form)) save.mutate()
          }}
          className="space-y-6"
        >
          <ServiceFormFields form={form} onChange={setForm} mode="edit" />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!serviceFormValid(form) || save.isPending}>
              {save.isPending ? "Saving…" : "Save changes"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function DeleteDialog({ service }: { service: Service }) {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)

  const del = useMutation({
    mutationFn: () => api.deleteService(service.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["services"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success(`Service “${service.name}” deleted`)
      navigate({ to: "/services" })
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't delete the service"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="ghost" className="text-muted-foreground hover:text-red-500">
          <Trash2 className="size-4" /> Delete
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete service?</DialogTitle>
          <DialogDescription>
            <Mono>{service.public_hostname}</Mono> will stop being served and its route is removed from the edge. This
            can't be undone (but you can roll back the config snapshot).
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => setOpen(false)}>
            Cancel
          </Button>
          <Button
            className="bg-red-600 text-white hover:bg-red-600/90"
            disabled={del.isPending}
            onClick={() => del.mutate()}
          >
            {del.isPending ? "Deleting…" : "Delete service"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
