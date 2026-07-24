import { useState, type ReactNode } from "react"
import { Plus, X } from "lucide-react"
import type { HTTPConfig, Service, ServiceUpdate, WafMode } from "@/lib/api"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { TokenInput } from "@/components/ui/token-input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

// ── Form state ──────────────────────────────────────────────────────────────
// One shape drives both the create and edit pages so they can never drift.

export type ServiceFormState = {
  name: string
  hostnames: string[]
  upstreams: string[]
  tlsMode: string
  lbPolicy: string
  wafEnabled: boolean
  wafMode: "" | WafMode
  http: HTTPConfig
  rawCaddy: string
  enabled: boolean
}

export function emptyServiceForm(): ServiceFormState {
  return {
    name: "",
    hostnames: [],
    upstreams: [],
    tlsMode: "managed",
    lbPolicy: "round_robin",
    wafEnabled: true,
    wafMode: "",
    http: {},
    rawCaddy: "",
    enabled: true,
  }
}

export function serviceToForm(s: Service): ServiceFormState {
  return {
    name: s.name,
    hostnames: s.public_hostnames,
    upstreams: s.upstreams,
    tlsMode: s.tls_mode,
    lbPolicy: s.lb_policy,
    wafEnabled: s.waf_enabled,
    wafMode: s.waf_mode,
    http: s.http ?? {},
    rawCaddy: s.raw_caddy ?? "",
    enabled: s.enabled,
  }
}

export function formToInput(f: ServiceFormState): ServiceUpdate {
  return {
    name: f.name.trim(),
    public_hostnames: f.hostnames,
    upstreams: f.upstreams,
    tls_mode: f.tlsMode,
    lb_policy: f.lbPolicy,
    waf_enabled: f.wafEnabled,
    waf_mode: f.wafEnabled ? f.wafMode : "",
    http: f.http,
    raw_caddy: f.rawCaddy.trim() || undefined,
    enabled: f.enabled,
  }
}

export function serviceFormValid(f: ServiceFormState): boolean {
  return f.name.trim().length > 0 && f.hostnames.length > 0 && f.upstreams.length > 0
}

// ── Layout primitives ─────────────────────────────────────────────────────────

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-4">
      <h3 className="font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{title}</h3>
      {children}
    </section>
  )
}

function Field({
  label,
  hint,
  htmlFor,
  children,
}: {
  label: string
  hint?: string
  htmlFor?: string
  children: ReactNode
}) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={htmlFor}>
        {label}
        {hint && <span className="ml-1.5 font-normal text-muted-foreground">{hint}</span>}
      </Label>
      {children}
    </div>
  )
}

function SubLabel({ children }: { children: ReactNode }) {
  return <div className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground/80">{children}</div>
}

// HeaderEditor edits a header map as position-keyed rows (typing a key never remounts).
function HeaderEditor({
  value,
  onChange,
}: {
  value?: Record<string, string>
  onChange: (v: Record<string, string>) => void
}) {
  const [rows, setRows] = useState<{ k: string; v: string }[]>(Object.entries(value ?? {}).map(([k, v]) => ({ k, v })))
  const sync = (next: { k: string; v: string }[]) => {
    setRows(next)
    onChange(Object.fromEntries(next.filter((r) => r.k.trim()).map((r) => [r.k.trim(), r.v])))
  }
  return (
    <div className="space-y-1.5">
      {rows.map((r, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            className="h-9 flex-1 font-mono text-xs"
            placeholder="Header"
            value={r.k}
            onChange={(e) => sync(rows.map((x, j) => (j === i ? { ...x, k: e.target.value } : x)))}
          />
          <span className="text-muted-foreground/50">:</span>
          <Input
            className="h-9 flex-1 font-mono text-xs"
            placeholder="value"
            value={r.v}
            onChange={(e) => sync(rows.map((x, j) => (j === i ? { ...x, v: e.target.value } : x)))}
          />
          <button
            type="button"
            aria-label="Remove header"
            onClick={() => sync(rows.filter((_, j) => j !== i))}
            className="text-muted-foreground transition-colors hover:text-red-500"
          >
            <X className="size-4" />
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => sync([...rows, { k: "", v: "" }])}
        className="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        <Plus className="size-3" /> Add header
      </button>
    </div>
  )
}

// ── The shared form ───────────────────────────────────────────────────────────

const LB_OPTIONS = [
  { value: "round_robin", label: "Round robin" },
  { value: "least_conn", label: "Least connections" },
  { value: "random", label: "Random" },
  { value: "ip_hash", label: "IP hash · sticky" },
]
const TLS_OPTIONS = [
  { value: "managed", label: "Managed · Let's Encrypt" },
  { value: "internal", label: "Internal CA · self-signed" },
  { value: "custom", label: "Custom certificate · upload" },
  { value: "none", label: "None · HTTP only" },
]

// ServiceFormFields lays the whole form out across two columns on a page: the core
// service config on the left, and HTTP options as a full column on the right — so
// the header editors and the raw-Caddyfile box get real horizontal room instead of a
// cramped, endlessly-scrolling modal. Below `lg` the two columns stack.
export function ServiceFormFields({
  form,
  onChange,
  mode,
}: {
  form: ServiceFormState
  onChange: (f: ServiceFormState) => void
  mode: "create" | "edit"
}) {
  const set = (patch: Partial<ServiceFormState>) => onChange({ ...form, ...patch })
  const h = form.http ?? {}
  const setHttp = (patch: Partial<HTTPConfig>) => set({ http: { ...h, ...patch } })

  return (
    <div className="grid gap-y-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
      {/* LEFT — core */}
      <div className="space-y-8 lg:pr-10">
        <Section title="Identity">
          <Field label="Name" htmlFor="svc-name">
            <Input id="svc-name" value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder="app-api" />
          </Field>
          <Field label="Public hostnames" hint="— Enter to add each">
            <TokenInput
              ariaLabel="Public hostnames"
              value={form.hostnames}
              onChange={(hostnames) => set({ hostnames })}
              placeholder="api.acme.com"
            />
            <p className="mt-1 text-xs text-muted-foreground">All names route to this one service — one WAF policy, one set of rules.</p>
          </Field>
        </Section>

        <Section title="Backend">
          <Field label="Upstreams" hint="— host:port, Enter to add each">
            <TokenInput
              ariaLabel="Upstreams"
              value={form.upstreams}
              onChange={(upstreams) => set({ upstreams })}
              placeholder="api-1.mesh:8000"
            />
            <p className="mt-1 text-xs text-muted-foreground">Multiple upstreams are load-balanced replicas of the same app.</p>
          </Field>
          <Field label="Load balancing" htmlFor="svc-lb">
            <Select value={form.lbPolicy} onValueChange={(lbPolicy) => set({ lbPolicy })}>
              <SelectTrigger id="svc-lb" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LB_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </Section>

        <Section title="TLS & protection">
          <Field label="TLS" htmlFor="svc-tls">
            <Select value={form.tlsMode} onValueChange={(tlsMode) => set({ tlsMode })}>
              <SelectTrigger id="svc-tls" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {TLS_OPTIONS.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="WAF" htmlFor="svc-wafmode">
            <label className="flex h-9 items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="size-4 accent-primary"
                checked={form.wafEnabled}
                onChange={(e) => set({ wafEnabled: e.target.checked })}
              />
              Protect with Coraza + OWASP CRS
            </label>
            {form.wafEnabled && (
              <Select
                value={form.wafMode || "inherit"}
                onValueChange={(v) => set({ wafMode: v === "inherit" ? "" : (v as WafMode) })}
              >
                <SelectTrigger id="svc-wafmode" className="mt-1.5 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inherit">Inherit global default</SelectItem>
                  <SelectItem value="DetectionOnly">Detection only</SelectItem>
                  <SelectItem value="On">Enforcing · 403 on attack</SelectItem>
                </SelectContent>
              </Select>
            )}
          </Field>
        </Section>

        {mode === "edit" && (
          <label className="flex items-center gap-2.5 border-t pt-6 text-sm">
            <input
              type="checkbox"
              className="size-4 accent-primary"
              checked={form.enabled}
              onChange={(e) => set({ enabled: e.target.checked })}
            />
            Enabled (serving traffic)
          </label>
        )}
      </div>

      {/* RIGHT — HTTP options */}
      <div className="lg:border-l lg:pl-10">
        <Section title="HTTP options">
          {/* Headers */}
          <div className="space-y-4">
            <SubLabel>Headers</SubLabel>
            <label className="flex items-start gap-2 text-sm">
              <input
                type="checkbox"
                className="mt-0.5 accent-primary"
                checked={!!h.security_headers}
                onChange={(e) => setHttp({ security_headers: e.target.checked })}
              />
              <span>
                Security headers preset{" "}
                <span className="text-xs text-muted-foreground">— HSTS, X-Frame-Options, nosniff, Referrer-Policy</span>
              </span>
            </label>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Response headers (sent to the client)</Label>
              <HeaderEditor value={h.response_headers} onChange={(v) => setHttp({ response_headers: v })} />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Request headers (sent to the upstream)</Label>
              <HeaderEditor value={h.request_headers} onChange={(v) => setHttp({ request_headers: v })} />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Strip response headers</Label>
              <TokenInput
                value={h.remove_headers ?? []}
                onChange={(v) => setHttp({ remove_headers: v })}
                placeholder="Server"
                ariaLabel="Strip response headers"
              />
            </div>
          </div>

          {/* Access · Path · Transfer */}
          <div className="grid gap-6 border-t pt-5 sm:grid-cols-2">
            <div className="space-y-2 sm:col-span-2">
              <SubLabel>Access — basic auth</SubLabel>
              <div className="grid grid-cols-2 gap-3">
                <Input
                  className="h-9"
                  placeholder="username"
                  value={h.basic_auth_user ?? ""}
                  onChange={(e) => setHttp({ basic_auth_user: e.target.value })}
                />
                <Input
                  className="h-9"
                  type="password"
                  placeholder={mode === "edit" ? "leave blank to keep" : "password"}
                  value={h.basic_auth_password ?? ""}
                  onChange={(e) => setHttp({ basic_auth_password: e.target.value })}
                />
              </div>
              <p className="text-[11px] text-muted-foreground">Clear the username to turn auth off.</p>
            </div>
            <div className="space-y-1.5">
              <SubLabel>Path</SubLabel>
              <Input
                className="h-9 font-mono"
                placeholder="/api"
                value={h.strip_path_prefix ?? ""}
                onChange={(e) => setHttp({ strip_path_prefix: e.target.value })}
              />
              <p className="text-[11px] text-muted-foreground">Strip this prefix before proxying.</p>
            </div>
            <div className="space-y-1.5">
              <SubLabel>Transfer</SubLabel>
              <label className="flex h-9 items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="accent-primary"
                  checked={!!h.compression}
                  onChange={(e) => setHttp({ compression: e.target.checked })}
                />
                Compression <span className="text-xs text-muted-foreground">— gzip / zstd</span>
              </label>
            </div>
          </div>

          {/* Advanced */}
          <div className="space-y-1.5 border-t pt-5">
            <SubLabel>Advanced — raw Caddyfile</SubLabel>
            <textarea
              value={form.rawCaddy}
              onChange={(e) => set({ rawCaddy: e.target.value })}
              rows={5}
              spellCheck={false}
              placeholder={"redir /old /new 302\n# the reverse_proxy is added automatically"}
              className="w-full resize-y rounded-md border bg-background px-3 py-2 font-mono text-xs leading-relaxed shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
            <p className="text-[11px] text-muted-foreground">
              For what the fields above can't do. Runs just before the proxy; validated on save — a syntax error is
              rejected.
            </p>
          </div>
        </Section>
      </div>
    </div>
  )
}
