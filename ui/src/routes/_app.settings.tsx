import { useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { AlertTriangle, Moon, Sun } from "lucide-react"
import { cn } from "@/lib/utils"
import { PageHeader, StatusDot, Mono } from "@/components/console"
import { useTheme } from "@/lib/theme"
import { api, ApiError } from "@/lib/api"
import type { WafMode } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

export const Route = createFileRoute("/_app/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const { theme, toggle } = useTheme()

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Admin" title="Settings" description="Edge-wide configuration. Changes are recorded in the audit log." />

      <div className="max-w-3xl space-y-6">
        <WafEngineSection />

        <RulesetSection />

        <EdgeSection />

        <TlsSection />

        <RetentionSection />

        <WafRetentionSection />

        <CrowdSecSection />

        <Section title="Appearance" description="Theme for this browser.">
          <Segmented
            options={[
              { value: "light", label: "Light" },
              { value: "dark", label: "Dark" },
            ]}
            value={theme}
            onChange={(v) => {
              if (v !== theme) toggle()
            }}
            icons={{ light: <Sun className="size-3.5" />, dark: <Moon className="size-3.5" /> }}
          />
        </Section>
      </div>
    </div>
  )
}

function WafEngineSection() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const mode = data?.waf_engine_mode ?? "DetectionOnly"
  const update = useMutation({
    mutationFn: (m: WafMode) => api.updateSettings({ waf_engine_mode: m }),
    onSuccess: (s) => {
      qc.setQueryData(["settings"], s)
      toast.success(
        s.waf_engine_mode === "On" ? "WAF now enforcing — attacks get a 403" : "WAF back to detection-only",
      )
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't change the WAF mode"),
  })

  return (
    <Section
      title="WAF engine default"
      description="The default enforcement mode for every protected service. Any service can override it on its own page."
    >
      {isLoading ? (
        <Skeleton className="h-9 w-56" />
      ) : (
        <>
          <Segmented
            options={[
              { value: "DetectionOnly", label: "Detection only" },
              { value: "On", label: "Enforcing" },
            ]}
            value={mode}
            onChange={(v) => {
              if (v !== mode) update.mutate(v as WafMode)
            }}
          />
          {mode === "On" ? (
            <p className="flex items-start gap-2 text-xs text-amber-500">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
              Attacks get a 403 on every service that inherits this. Tune false positives first — check Top Triggers.
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              The WAF logs detections but lets requests through. Soak here until the false positives are gone, then enforce.
            </p>
          )}
        </>
      )}
    </Section>
  )
}

function RulesetSection() {
  const { data } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const crs = data?.crs_version
  const label = crs ? crs.replace(/^OWASP_CRS\//, "OWASP CRS ") : null
  return (
    <Section
      title="WAF ruleset"
      description="The OWASP Core Rule Set is compiled into the edge image — update it by pulling a newer Ward release, not a separate rules download."
    >
      {label ? (
        <span className="inline-flex w-fit items-center rounded-md border bg-muted/40 px-2 py-1 font-mono text-xs">
          {label}
        </span>
      ) : (
        <p className="text-xs text-muted-foreground">
          The running version appears here once the WAF logs its first detection.
        </p>
      )}
    </Section>
  )
}

// EdgeSection lists the components compiled into the ward-caddy image this release
// ships. Read-only — they move together with the Ward version. Values come from the
// image's pinned build (kept in lockstep with caddy/Dockerfile); the authoritative
// source for a running image is `docker inspect` (its OCI labels).
function EdgeSection() {
  const { data } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const versions = data?.edge_versions ?? {}
  const rows: [string, string][] = [
    ["Caddy", "caddy"],
    ["Coraza", "coraza"],
    ["OWASP CRS", "crs"],
    ["CrowdSec bouncer", "crowdsec_bouncer"],
    ["Rate limit", "ratelimit"],
    ["GeoIP", "maxmind_geolocation"],
  ]
  const present = rows.filter(([, k]) => versions[k])
  return (
    <Section
      title="Edge build"
      description="The components compiled into the ward-caddy image this release runs. They move together — update by pulling a newer Ward release. For a running image, docker inspect shows the exact versions as OCI labels."
    >
      {present.length ? (
        <div className="flex flex-wrap gap-2">
          {present.map(([label, k]) => (
            <span key={k} className="inline-flex items-center gap-1.5 rounded-md border bg-muted/40 px-2 py-1 text-xs">
              <span className="text-muted-foreground">{label}</span>
              <Mono className="!text-xs">{versions[k]}</Mono>
            </span>
          ))}
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">Unavailable.</p>
      )}
    </Section>
  )
}

function TlsSection() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const [draft, setDraft] = useState<string | null>(null)
  const email = draft ?? data?.acme_email ?? ""
  const save = useMutation({
    mutationFn: () => api.updateSettings({ acme_email: email.trim() }),
    onSuccess: (s) => {
      qc.setQueryData(["settings"], s)
      setDraft(null)
      toast.success("ACME email saved")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save the email"),
  })

  return (
    <Section
      title="TLS"
      description="Contact email for managed (Let's Encrypt) certificates — used as the ACME account email. Services with a certificate redirect HTTP→HTTPS automatically."
      onSave={() => {
        if (email.trim()) save.mutate()
      }}
    >
      {isLoading ? (
        <Skeleton className="h-9 max-w-sm" />
      ) : (
        <Input
          className="max-w-sm font-mono"
          type="email"
          value={email}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="ops@acme.com"
        />
      )}
    </Section>
  )
}

function RetentionSection() {
  const qc = useQueryClient()
  const { data } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const days = data?.access_retention_days ?? 7
  const save = useMutation({
    mutationFn: (d: number) => api.updateSettings({ access_retention_days: d }),
    onSuccess: (s) => {
      qc.setQueryData(["settings"], s)
      toast.success(`Access log kept ${s.access_retention_days} days`)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save"),
  })
  return (
    <Section
      title="Access-log retention"
      description="How long raw access events are kept in Ward for the Access Log view. Ship the log to Loki/Grafana for longer, searchable history."
    >
      <Select value={String(days)} onValueChange={(v) => save.mutate(Number(v))}>
        <SelectTrigger className="w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="1">1 day</SelectItem>
          <SelectItem value="3">3 days</SelectItem>
          <SelectItem value="7">7 days</SelectItem>
        </SelectContent>
      </Select>
    </Section>
  )
}

function WafRetentionSection() {
  const qc = useQueryClient()
  const { data } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const days = data?.waf_retention_days ?? 30
  const save = useMutation({
    mutationFn: (d: number) => api.updateSettings({ waf_retention_days: d }),
    onSuccess: (s) => {
      qc.setQueryData(["settings"], s)
      toast.success(`WAF detections kept ${s.waf_retention_days} days`)
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't save"),
  })
  return (
    <Section
      title="WAF-event retention"
      description="How long WAF detections are kept. They feed Top Triggers and exclusion decisions, so keep enough history to tune against."
    >
      <Select value={String(days)} onValueChange={(v) => save.mutate(Number(v))}>
        <SelectTrigger className="w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="7">7 days</SelectItem>
          <SelectItem value="30">30 days</SelectItem>
          <SelectItem value="90">90 days</SelectItem>
        </SelectContent>
      </Select>
    </Section>
  )
}

function Section({
  title,
  description,
  children,
  onSave,
}: {
  title: string
  description: string
  children: React.ReactNode
  onSave?: () => void
}) {
  return (
    <section className="rounded-xl border bg-card">
      <div className="flex flex-col gap-4 p-5 sm:flex-row sm:items-start sm:justify-between">
        <div className="max-w-xs space-y-1">
          <h2 className="font-heading text-sm font-semibold">{title}</h2>
          <p className="text-xs text-muted-foreground">{description}</p>
        </div>
        <div className="flex-1 space-y-2 sm:max-w-sm">{children}</div>
      </div>
      {onSave && (
        <div className="flex justify-end border-t px-5 py-3">
          <Button size="sm" onClick={onSave}>
            Save
          </Button>
        </div>
      )}
    </section>
  )
}

function CrowdSecSection() {
  const qc = useQueryClient()
  const { data: settings } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const { data: status } = useQuery({ queryKey: ["crowdsec"], queryFn: api.crowdsecStatus, refetchInterval: 10000 })
  const configured = settings?.crowdsec_configured ?? false
  const enabled = settings?.crowdsec_enabled ?? false
  const toggle = useMutation({
    mutationFn: (on: boolean) => api.updateSettings({ crowdsec_enabled: on }),
    onSuccess: (s) => {
      qc.setQueryData(["settings"], s)
      qc.invalidateQueries({ queryKey: ["crowdsec"] })
      toast.success(s.crowdsec_enabled ? "CrowdSec bouncer enabled at the edge" : "CrowdSec bouncer disabled")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't change CrowdSec"),
  })
  const count = status?.decisions?.length ?? 0
  return (
    <Section
      title="CrowdSec"
      description="Community IP reputation + behavioral blocking. The agent detects from the access log, the edge bouncer enforces bans, and Ward observes decisions read-only — never in the request path."
    >
      {!configured ? (
        <p className="text-xs text-muted-foreground">
          Not configured. Set <Mono className="!text-[11px]">WARD_CROWDSEC_API_URL</Mono> and{" "}
          <Mono className="!text-[11px]">WARD_CROWDSEC_API_KEY</Mono> in the deployment, then reload.
        </p>
      ) : (
        <div className="space-y-3">
          <Segmented
            options={[
              { value: "on", label: "Enabled" },
              { value: "off", label: "Disabled" },
            ]}
            value={enabled ? "on" : "off"}
            onChange={(v) => {
              const on = v === "on"
              if (on !== enabled) toggle.mutate(on)
            }}
          />
          {enabled && (
            <p className="flex items-center gap-2 text-xs">
              <StatusDot tone={status?.reachable ? "ok" : "threat"} />
              {status?.reachable ? (
                <span className="text-muted-foreground">
                  LAPI reachable · {count} active decision{count === 1 ? "" : "s"} —{" "}
                  <Link to="/blocklist" className="underline hover:text-foreground">
                    view in Blocklist
                  </Link>
                </span>
              ) : (
                <span className="text-red-500">LAPI unreachable{status?.error ? ` — ${status.error}` : ""}</span>
              )}
            </p>
          )}
        </div>
      )}
    </Section>
  )
}

function Segmented({
  options,
  value,
  onChange,
  icons,
}: {
  options: { value: string; label: string }[]
  value: string
  onChange: (v: string) => void
  icons?: Record<string, React.ReactNode>
}) {
  return (
    <div className="inline-flex rounded-md border bg-background p-0.5">
      {options.map((o) => (
        <button
          key={o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            "inline-flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium transition-colors",
            value === o.value ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {icons?.[o.value]}
          {o.label}
        </button>
      ))}
    </div>
  )
}
