import { useRef, useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { FileKey, Trash2, Upload, AlertTriangle } from "lucide-react"
import { PageHeader, StatusDot, Mono, ago } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { Certificate } from "@/lib/api"
import { useServices } from "@/data/queries"
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

export const Route = createFileRoute("/_app/certificates")({
  component: CertificatesPage,
})

const DAY = 86_400_000

// Days until expiry (negative = already expired).
// Mirrors the backend certs.SANMatches: a subject (CN/SAN) secures host, incl. a
// single-label wildcard (*.example.com → a.example.com). Used to mark a cert "in use"
// when a custom-mode service's hostname is covered by the cert's SAN, not just its
// storage-folder domain.
function sanMatches(host: string, san: string): boolean {
  host = host.toLowerCase().trim()
  san = san.toLowerCase().trim()
  if (!host || !san) return false
  if (san === host) return true
  if (san.startsWith("*.")) {
    const suffix = san.slice(1) // ".example.com"
    if (host.endsWith(suffix)) {
      const label = host.slice(0, host.length - suffix.length)
      return label.length > 0 && !label.includes(".")
    }
  }
  return false
}

function daysLeft(iso: string): number {
  return Math.floor((new Date(iso).getTime() - Date.now()) / DAY)
}

function readAsText(f: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onload = () => resolve(String(r.result))
    r.onerror = () => reject(new Error("couldn't read the file"))
    r.readAsText(f)
  })
}

function CertificatesPage() {
  const qc = useQueryClient()
  const { data: certs, isLoading, error } = useQuery({ queryKey: ["certificates"], queryFn: api.listCertificates })
  const { data: services } = useServices()
  const remove = useMutation({
    mutationFn: (domain: string) => api.deleteCertificate(domain),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["certificates"] })
      toast.success("Certificate removed")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove the certificate"),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Edge"
        title="Certificates"
        description="Upload your own TLS certificate for a hostname. A service set to “Custom certificate” TLS serves the matching cert. (Let’s Encrypt and self-signed certs are issued automatically — no upload needed.)"
        actions={<UploadDialog />}
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Domain</th>
              <th className="px-4 py-2.5 font-medium">Used by</th>
              <th className="px-4 py-2.5 font-medium">Covers</th>
              <th className="px-4 py-2.5 font-medium">Expires</th>
              <th className="px-4 py-2.5 font-medium">Uploaded</th>
              <th className="w-10" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading && (
              <tr>
                <td colSpan={6} className="px-4 py-3.5">
                  <Skeleton className="h-6 w-full" />
                </td>
              </tr>
            )}
            {error && (
              <tr>
                <td colSpan={6} className="py-12 text-center text-sm text-red-500">
                  Couldn't load certificates.
                </td>
              </tr>
            )}
            {certs?.length === 0 && (
              <tr>
                <td colSpan={6} className="py-16 text-center text-sm text-muted-foreground">
                  No custom certificates. Upload one to serve it on a “Custom certificate” service.
                </td>
              </tr>
            )}
            {certs?.map((c) => {
              const d = daysLeft(c.not_after)
              const tone = d < 0 ? "threat" : d < 30 ? "detecting" : "ok"
              // In use if any custom-mode service's hostname is covered by the cert's SAN
              // (matches config-gen's SAN loading — not just the storage-folder domain).
              const usedByServices = (services ?? []).filter(
                (s) => s.tls_mode === "custom" && (c.subjects ?? []).some((san) => sanMatches(s.public_hostname, san)),
              )
              return (
                <tr key={c.domain} className="group transition-colors hover:bg-muted/40">
                  <td className="px-4 py-3">
                    <Mono className="font-medium">{c.domain}</Mono>
                  </td>
                  <td className="px-4 py-3">
                    {usedByServices.length > 0 ? (
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                        {usedByServices.map((s) => (
                          <Link
                            key={s.id}
                            to="/services/$id"
                            params={{ id: s.id }}
                            className="inline-flex items-center gap-1.5 hover:underline"
                          >
                            <StatusDot tone="ok" />
                            <span className="text-xs">{s.name}</span>
                          </Link>
                        ))}
                      </div>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                        <StatusDot tone="idle" />
                        <span className="text-xs">unused</span>
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {c.subjects.map((s) => (
                        <span
                          key={s}
                          className="rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
                        >
                          {s}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1.5">
                      <StatusDot tone={tone} />
                      <Mono dim className="!text-xs">
                        {d < 0 ? "expired" : `${new Date(c.not_after).toISOString().slice(0, 10)} · ${d}d`}
                      </Mono>
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <Mono dim className="!text-xs">
                      {ago(c.updated_at)}
                    </Mono>
                  </td>
                  <td className="pr-3">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                      aria-label="Remove certificate"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(c.domain)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function UploadDialog() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [domain, setDomain] = useState("")
  const [certFile, setCertFile] = useState<File | null>(null)
  const [keyFile, setKeyFile] = useState<File | null>(null)
  const certRef = useRef<HTMLInputElement>(null)
  const keyRef = useRef<HTMLInputElement>(null)

  const upload = useMutation({
    mutationFn: async () => {
      const [cert_pem, key_pem] = await Promise.all([readAsText(certFile!), readAsText(keyFile!)])
      return api.uploadCertificate({ domain: domain.trim(), cert_pem, key_pem })
    },
    onSuccess: (c: Certificate) => {
      qc.invalidateQueries({ queryKey: ["certificates"] })
      toast.success(`Certificate for ${c.domain} uploaded`)
      setOpen(false)
      setDomain("")
      setCertFile(null)
      setKeyFile(null)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't upload the certificate"),
  })

  const ready = domain.trim() && certFile && keyFile

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Upload className="size-4" /> Upload certificate
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Upload a certificate</DialogTitle>
          <DialogDescription>
            PEM cert + private key for one hostname. Ward validates the pair covers the domain, stores it on the certs
            volume, and serves it on any “Custom certificate” service for that host.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (ready) upload.mutate()
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="cert-domain">Domain</Label>
            <Input
              id="cert-domain"
              className="font-mono"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="api.acme.com"
            />
            <p className="text-xs text-muted-foreground">The hostname this cert secures — match the service's public hostname.</p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <PemPicker
              label="Certificate"
              hint=".pem / .crt (full chain)"
              file={certFile}
              inputRef={certRef}
              accept=".pem,.crt,.cer"
              onPick={setCertFile}
            />
            <PemPicker
              label="Private key"
              hint=".pem / .key"
              file={keyFile}
              inputRef={keyRef}
              accept=".pem,.key"
              onPick={setKeyFile}
            />
          </div>
          <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-600 dark:text-amber-400">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
            The private key is stored on the certs volume (file-permission protected). Keep it off shared channels.
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!ready || upload.isPending}>
              {upload.isPending ? "Uploading…" : "Upload"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function PemPicker({
  label,
  hint,
  file,
  inputRef,
  accept,
  onPick,
}: {
  label: string
  hint: string
  file: File | null
  inputRef: React.RefObject<HTMLInputElement | null>
  accept: string
  onPick: (f: File | null) => void
}) {
  return (
    <div className="space-y-1.5">
      <Label>{label}</Label>
      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        className="flex h-9 w-full items-center gap-2 rounded-md border bg-background px-3 text-left text-sm transition-colors hover:bg-muted/40"
      >
        <FileKey className="size-4 shrink-0 text-muted-foreground" />
        <span className="min-w-0 flex-1 truncate font-mono text-xs">{file ? file.name : "Choose file…"}</span>
      </button>
      <p className="text-[11px] text-muted-foreground">{hint}</p>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        className="hidden"
        onChange={(e) => onPick(e.target.files?.[0] ?? null)}
      />
    </div>
  )
}
