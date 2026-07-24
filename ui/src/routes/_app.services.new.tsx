import { useState } from "react"
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { ArrowLeft } from "lucide-react"
import { PageHeader } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import {
  ServiceFormFields,
  emptyServiceForm,
  formToInput,
  serviceFormValid,
  type ServiceFormState,
} from "@/components/service-form"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/_app/services/new")({
  component: NewServicePage,
})

function NewServicePage() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [form, setForm] = useState<ServiceFormState>(emptyServiceForm())

  const create = useMutation({
    mutationFn: () => api.createService(formToInput(form)),
    onSuccess: (svc) => {
      qc.invalidateQueries({ queryKey: ["services"] })
      qc.invalidateQueries({ queryKey: ["overview"] })
      toast.success(`Service “${svc.name}” created`, { description: "Route generated and applied to the edge." })
      navigate({ to: "/services/$id", params: { id: svc.id } })
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't create the service"),
  })

  return (
    <div className="mx-auto max-w-5xl space-y-8">
      <div className="space-y-4">
        <Link
          to="/services"
          className="inline-flex items-center gap-1.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-3.5" /> Services
        </Link>
        <PageHeader
          eyebrow="Edge"
          title="New service"
          description="Ward generates the Caddy route, validates it, and applies it to the edge — with a snapshot you can roll back to."
        />
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (serviceFormValid(form)) create.mutate()
        }}
        className="space-y-8"
      >
        <ServiceFormFields form={form} onChange={setForm} mode="create" />
        <div className="flex items-center justify-end gap-2 border-t pt-6">
          <Button type="button" variant="ghost" onClick={() => navigate({ to: "/services" })}>
            Cancel
          </Button>
          <Button type="submit" disabled={!serviceFormValid(form) || create.isPending}>
            {create.isPending ? "Creating…" : "Create service"}
          </Button>
        </div>
      </form>
    </div>
  )
}
