import { useState } from "react"
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { ArrowLeft } from "lucide-react"
import { PageHeader } from "@/components/console"
import { api, ApiError, type Service } from "@/lib/api"
import {
  ServiceFormFields,
  serviceToForm,
  formToInput,
  serviceFormValid,
  type ServiceFormState,
} from "@/components/service-form"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"

export const Route = createFileRoute("/_app/services/$id_/edit")({
  component: EditServicePage,
})

function EditServicePage() {
  const { id } = Route.useParams()
  const { data: service, isLoading, error } = useQuery({ queryKey: ["service", id], queryFn: () => api.getService(id) })

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <div className="space-y-4">
        <Link
          to="/services/$id"
          params={{ id }}
          className="inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-3.5" /> {service?.name ?? "Service"}
        </Link>
        <PageHeader
          eyebrow="Edge"
          title={service ? `Edit ${service.name}` : "Edit service"}
          description="Changes regenerate the Caddy route and apply to the edge, with a rollback snapshot."
        />
      </div>

      {isLoading && <Skeleton className="h-96 w-full rounded-xl" />}
      {error && <p className="text-sm text-red-500">Couldn't load the service. Is the backend running?</p>}
      {service && <EditForm service={service} />}
    </div>
  )
}

function EditForm({ service }: { service: Service }) {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [form, setForm] = useState<ServiceFormState>(() => serviceToForm(service))

  const back = () => navigate({ to: "/services/$id", params: { id: service.id } })

  const save = useMutation({
    mutationFn: () => api.updateService(service.id, formToInput(form)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["service", service.id] })
      qc.invalidateQueries({ queryKey: ["services"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success("Service updated", { description: "Config regenerated and applied to the edge." })
      back()
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't update the service"),
  })

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        if (serviceFormValid(form)) save.mutate()
      }}
      className="space-y-8"
    >
      <ServiceFormFields form={form} onChange={setForm} mode="edit" />
      <div className="flex items-center justify-end gap-2 border-t pt-6">
        <Button type="button" variant="ghost" onClick={back}>
          Cancel
        </Button>
        <Button type="submit" disabled={!serviceFormValid(form) || save.isPending}>
          {save.isPending ? "Saving…" : "Save changes"}
        </Button>
      </div>
    </form>
  )
}
