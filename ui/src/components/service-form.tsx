import { useState, type ReactNode } from "react"
import { Plus, X } from "lucide-react"
import type { HTTPConfig, Service, ServiceUpdate, WafMode } from "@/lib/api"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
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
  wafSkipPaths: string[]
  http: HTTPConfig
  healthCheck: HealthCheckForm
  redirect: RedirectForm
  pathRules: PathRuleForm[]
  rawCaddy: string
  enabled: boolean
}

// PathRuleForm mirrors the API's PathRule; `action` drives the proxy/deny toggle.
export type PathRuleForm = {
  path: string
  match: "prefix" | "exact"
  action: "proxy" | "deny"
  upstreams: string[]
  stripPrefix: string
}

// HealthCheckForm mirrors the API's HealthCheck but keeps expect_status as a string for
// the input (parsed on submit).
export type HealthCheckForm = {
  active: boolean
  path: string
  interval: string
  timeout: string
  expectStatus: string
}

// RedirectForm mirrors the API's Redirect; `enabled` drives the proxy/redirect toggle
// (the service is a redirect when enabled + a target is set), status kept as a string.
export type RedirectForm = {
  enabled: boolean
  to: string
  status: string
  preservePath: boolean
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
    wafSkipPaths: [],
    http: {},
    healthCheck: { active: false, path: "", interval: "", timeout: "", expectStatus: "" },
    redirect: { enabled: false, to: "", status: "302", preservePath: true },
    pathRules: [],
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
    wafSkipPaths: s.waf_skip_paths ?? [],
    http: s.http ?? {},
    healthCheck: {
      active: !!s.health_check?.active,
      path: s.health_check?.path ?? "",
      interval: s.health_check?.interval ?? "",
      timeout: s.health_check?.timeout ?? "",
      expectStatus: s.health_check?.expect_status ? String(s.health_check.expect_status) : "",
    },
    redirect: {
      enabled: !!s.redirect?.to,
      to: s.redirect?.to ?? "",
      status: s.redirect?.status ? String(s.redirect.status) : "302",
      preservePath: s.redirect?.preserve_path ?? true,
    },
    pathRules: (s.path_rules ?? []).map((r) => ({
      path: r.path,
      match: r.match === "exact" ? "exact" : "prefix",
      action: r.deny ? "deny" : "proxy",
      upstreams: r.upstreams ?? [],
      stripPrefix: r.strip_prefix ?? "",
    })),
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
    waf_skip_paths: f.wafEnabled ? f.wafSkipPaths : [],
    http: f.http,
    health_check: {
      active: f.healthCheck.active,
      path: f.healthCheck.path.trim(),
      interval: f.healthCheck.interval.trim(),
      timeout: f.healthCheck.timeout.trim(),
      expect_status: f.healthCheck.expectStatus.trim() ? Number(f.healthCheck.expectStatus) : 0,
    },
    redirect: f.redirect.enabled
      ? { to: f.redirect.to.trim(), status: Number(f.redirect.status) || 302, preserve_path: f.redirect.preservePath }
      : {},
    path_rules: f.redirect.enabled
      ? []
      : f.pathRules
          .filter((r) => r.path.trim())
          .map((r) =>
            r.action === "deny"
              ? { path: r.path.trim(), match: r.match, deny: true }
              : { path: r.path.trim(), match: r.match, upstreams: r.upstreams, strip_prefix: r.stripPrefix.trim() || undefined },
          ),
    raw_caddy: f.rawCaddy.trim() || undefined,
    enabled: f.enabled,
  }
}

export function serviceFormValid(f: ServiceFormState): boolean {
  const backend = f.redirect.enabled ? f.redirect.to.trim().length > 0 : f.upstreams.length > 0
  // Every proxy path rule needs at least one upstream (deny rules don't).
  const rulesOk =
    f.redirect.enabled || f.pathRules.every((r) => !r.path.trim() || r.action === "deny" || r.upstreams.length > 0)
  return f.name.trim().length > 0 && f.hostnames.length > 0 && backend && rulesOk
}

// ── Layout primitives ─────────────────────────────────────────────────────────

// FormCard groups a set of fields under a mono micro-label — the console section
// header used across Ward. Sharp corners + a hairline border match the app language.
function FormCard({
  title,
  desc,
  children,
  className,
}: {
  title: string
  desc?: string
  children: ReactNode
  className?: string
}) {
  return (
    <section className={cn("border bg-card p-5 sm:p-6", className)}>
      <div className="mb-5">
        <h3 className="font-mono text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{title}</h3>
        {desc && <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{desc}</p>}
      </div>
      <div className="space-y-5">{children}</div>
    </section>
  )
}

function Field({ label, hint, htmlFor, children }: { label: string; hint?: string; htmlFor?: string; children: ReactNode }) {
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

// ToggleRow — a labelled Switch. The single on/off control used everywhere so every
// boolean option reads the same.
function ToggleRow({
  checked,
  onChange,
  label,
  hint,
  id,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  hint?: string
  id?: string
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <label htmlFor={id} className="cursor-pointer text-sm">
        {label}
        {hint && <span className="ml-1.5 text-xs text-muted-foreground">{hint}</span>}
      </label>
      <Switch id={id} checked={checked} onCheckedChange={onChange} aria-label={label} />
    </div>
  )
}

// AddButton — the single "add another row" control (headers, path rules) so they stay
// visually identical.
function AddButton({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <Button type="button" variant="outline" size="sm" onClick={onClick}>
      <Plus /> {children}
    </Button>
  )
}

// ModeToggle — sharp segmented control for the top-level proxy-vs-redirect choice.
function ModeToggle({ value, onChange }: { value: "proxy" | "redirect"; onChange: (v: "proxy" | "redirect") => void }) {
  const opts: { value: "proxy" | "redirect"; label: string }[] = [
    { value: "proxy", label: "Reverse proxy" },
    { value: "redirect", label: "Redirect" },
  ]
  return (
    <div className="inline-flex border bg-background p-0.5">
      {opts.map((o) => (
        <button
          key={o.value}
          type="button"
          onClick={() => onChange(o.value)}
          className={cn(
            "px-3.5 py-1 text-xs font-medium transition-colors",
            value === o.value ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

// HeaderEditor edits a header map as position-keyed rows (typing a key never remounts).
function HeaderEditor({ value, onChange }: { value?: Record<string, string>; onChange: (v: Record<string, string>) => void }) {
  const [rows, setRows] = useState<{ k: string; v: string }[]>(Object.entries(value ?? {}).map(([k, v]) => ({ k, v })))
  const sync = (next: { k: string; v: string }[]) => {
    setRows(next)
    onChange(Object.fromEntries(next.filter((r) => r.k.trim()).map((r) => [r.k.trim(), r.v])))
  }
  return (
    <div className="space-y-2">
      {rows.map((r, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            className="flex-1 font-mono"
            placeholder="Header"
            value={r.k}
            onChange={(e) => sync(rows.map((x, j) => (j === i ? { ...x, k: e.target.value } : x)))}
          />
          <span className="text-muted-foreground/40">:</span>
          <Input
            className="flex-1 font-mono"
            placeholder="value"
            value={r.v}
            onChange={(e) => sync(rows.map((x, j) => (j === i ? { ...x, v: e.target.value } : x)))}
          />
          <button
            type="button"
            aria-label="Remove header"
            onClick={() => sync(rows.filter((_, j) => j !== i))}
            className="grid size-8 shrink-0 place-items-center text-muted-foreground transition-colors hover:text-red-500"
          >
            <X className="size-4" />
          </button>
        </div>
      ))}
      <AddButton onClick={() => sync([...rows, { k: "", v: "" }])}>Add header</AddButton>
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

// ServiceFormFields is the shared create/edit body. A top bar carries the proxy/redirect
// mode and the enabled switch; the core config sits in a two-column grid (identity +
// backend on the left, TLS/protection + HTTP options on the right); and path routing gets
// its own full-width card below, where it has room to breathe.
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
  const setHC = (patch: Partial<HealthCheckForm>) => set({ healthCheck: { ...form.healthCheck, ...patch } })
  const setRedir = (patch: Partial<RedirectForm>) => set({ redirect: { ...form.redirect, ...patch } })
  const setRules = (pathRules: PathRuleForm[]) => set({ pathRules })
  const isRedirect = form.redirect.enabled

  return (
    <div className="space-y-6">
      {/* Top bar — mode + enabled */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <ModeToggle value={isRedirect ? "redirect" : "proxy"} onChange={(v) => setRedir({ enabled: v === "redirect" })} />
        {mode === "edit" && (
          <div className="flex items-center gap-2.5">
            <span className="text-sm">
              <span className="font-medium">{form.enabled ? "Enabled" : "Disabled"}</span>
              <span className="ml-1.5 text-xs text-muted-foreground">· serving traffic</span>
            </span>
            <Switch checked={form.enabled} onCheckedChange={(enabled) => set({ enabled })} aria-label="Enabled" />
          </div>
        )}
      </div>

      {/* 1 · Identity */}
      <FormCard title="Identity">
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label="Name" htmlFor="svc-name">
            <Input id="svc-name" value={form.name} onChange={(e) => set({ name: e.target.value })} placeholder="app-api" />
          </Field>
          <Field label="Public hostnames" hint="— Enter to add each">
            <TokenInput ariaLabel="Public hostnames" value={form.hostnames} onChange={(hostnames) => set({ hostnames })} placeholder="api.acme.com" />
          </Field>
        </div>
        <p className="text-xs text-muted-foreground">All names route to this one service — one WAF policy, one set of rules.</p>
      </FormCard>

      {/* 2 · Backend */}
      <FormCard
        title="Backend"
        desc={isRedirect ? "Answer with a 301/302 redirect instead of proxying." : "Requests are load-balanced across these upstreams."}
      >
        {isRedirect ? (
          <>
            <Field label="Redirect to" htmlFor="svc-redir-to">
              <Input
                id="svc-redir-to"
                className="font-mono"
                placeholder="https://new.example.com"
                value={form.redirect.to}
                onChange={(e) => setRedir({ to: e.target.value })}
              />
            </Field>
            <div className="grid gap-5 sm:grid-cols-2 sm:items-end">
              <Field label="Status" htmlFor="svc-redir-status">
                <Select value={form.redirect.status} onValueChange={(status) => setRedir({ status })}>
                  <SelectTrigger id="svc-redir-status" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="302">302 · temporary</SelectItem>
                    <SelectItem value="301">301 · permanent</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <div className="pb-1">
                <ToggleRow
                  label="Preserve path + query"
                  hint="— append the original path"
                  checked={form.redirect.preservePath}
                  onChange={(preservePath) => setRedir({ preservePath })}
                />
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label="Upstreams" hint="— host:port, Enter to add each">
                <TokenInput ariaLabel="Upstreams" value={form.upstreams} onChange={(upstreams) => set({ upstreams })} placeholder="api-1.mesh:8000" />
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
            </div>
            <p className="text-xs text-muted-foreground">Multiple upstreams are load-balanced replicas of the same app.</p>
            <div className="space-y-3 border-t pt-5">
              <ToggleRow
                label="Active health checks"
                hint="— periodic upstream probe"
                checked={form.healthCheck.active}
                onChange={(active) => setHC({ active })}
              />
              {form.healthCheck.active && (
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                  <div className="space-y-1">
                    <SubLabel>Path</SubLabel>
                    <Input className="font-mono" placeholder="/health" value={form.healthCheck.path} onChange={(e) => setHC({ path: e.target.value })} />
                  </div>
                  <div className="space-y-1">
                    <SubLabel>Expect status</SubLabel>
                    <Input className="font-mono" placeholder="200" value={form.healthCheck.expectStatus} onChange={(e) => setHC({ expectStatus: e.target.value })} />
                  </div>
                  <div className="space-y-1">
                    <SubLabel>Interval</SubLabel>
                    <Input className="font-mono" placeholder="10s" value={form.healthCheck.interval} onChange={(e) => setHC({ interval: e.target.value })} />
                  </div>
                  <div className="space-y-1">
                    <SubLabel>Timeout</SubLabel>
                    <Input className="font-mono" placeholder="5s" value={form.healthCheck.timeout} onChange={(e) => setHC({ timeout: e.target.value })} />
                  </div>
                </div>
              )}
              <p className="text-xs text-muted-foreground">Passive checks (shedding a failing upstream) are always on.</p>
            </div>
          </>
        )}
      </FormCard>

      {/* 3 · TLS & protection */}
      <FormCard title="TLS & protection">
        <div className="grid gap-x-8 gap-y-5 sm:grid-cols-2">
          <Field label="TLS mode" htmlFor="svc-tls">
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
          <div className="space-y-3">
            <ToggleRow
              label="Web Application Firewall"
              hint="— Coraza + OWASP CRS"
              checked={form.wafEnabled}
              onChange={(wafEnabled) => set({ wafEnabled })}
            />
            {form.wafEnabled && (
              <>
                <div className="space-y-1.5">
                  <SubLabel>Mode</SubLabel>
                  <Select
                    value={form.wafMode || "inherit"}
                    onValueChange={(v) => set({ wafMode: v === "inherit" ? "" : (v as WafMode) })}
                  >
                    <SelectTrigger id="svc-wafmode" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="inherit">Inherit global default</SelectItem>
                      <SelectItem value="DetectionOnly">Detection only</SelectItem>
                      <SelectItem value="On">Enforcing · 403 on attack</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <SubLabel>Skip paths — for streaming / SSE</SubLabel>
                  <TokenInput ariaLabel="WAF skip paths" value={form.wafSkipPaths} onChange={(wafSkipPaths) => set({ wafSkipPaths })} placeholder="/sse" />
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    The WAF is bypassed for these paths so SSE can stream (it buffers responses otherwise). Matches the path and its
                    subpaths. WebSocket upgrades bypass automatically. IP, geo, and rate-limit still apply.
                  </p>
                </div>
              </>
            )}
          </div>
        </div>
      </FormCard>

      {/* 4 · HTTP options */}
      <FormCard title="HTTP options">
        <ToggleRow
          label="Security headers preset"
          hint="— HSTS, X-Frame-Options, nosniff, Referrer-Policy"
          checked={!!h.security_headers}
          onChange={(v) => setHttp({ security_headers: v })}
        />
        <div className="grid gap-x-8 gap-y-5 sm:grid-cols-2">
          <div className="space-y-1.5">
            <SubLabel>Response headers — sent to the client</SubLabel>
            <HeaderEditor value={h.response_headers} onChange={(v) => setHttp({ response_headers: v })} />
          </div>
          <div className="space-y-1.5">
            <SubLabel>Request headers — sent to the upstream</SubLabel>
            <HeaderEditor value={h.request_headers} onChange={(v) => setHttp({ request_headers: v })} />
          </div>
          <div className="space-y-1.5">
            <SubLabel>Strip response headers</SubLabel>
            <TokenInput value={h.remove_headers ?? []} onChange={(v) => setHttp({ remove_headers: v })} placeholder="Server" ariaLabel="Strip response headers" />
          </div>
          <div className="space-y-1.5">
            <SubLabel>Strip path prefix</SubLabel>
            <Input className="font-mono" placeholder="/api" value={h.strip_path_prefix ?? ""} onChange={(e) => setHttp({ strip_path_prefix: e.target.value })} />
            <p className="text-[11px] text-muted-foreground">Stripped before proxying.</p>
          </div>
          <div className="space-y-2">
            <SubLabel>Access — basic auth</SubLabel>
            <div className="grid grid-cols-2 gap-3">
              <Input placeholder="username" value={h.basic_auth_user ?? ""} onChange={(e) => setHttp({ basic_auth_user: e.target.value })} />
              <Input
                type="password"
                placeholder={mode === "edit" ? "leave blank to keep" : "password"}
                value={h.basic_auth_password ?? ""}
                onChange={(e) => setHttp({ basic_auth_password: e.target.value })}
              />
            </div>
            <p className="text-[11px] text-muted-foreground">Clear the username to turn auth off.</p>
          </div>
          <div className="space-y-1.5">
            <SubLabel>Transfer</SubLabel>
            <div className="flex h-8 items-center">
              <ToggleRow label="Compression" hint="— gzip / zstd" checked={!!h.compression} onChange={(v) => setHttp({ compression: v })} />
            </div>
          </div>
        </div>
      </FormCard>

      {/* 5 · Path routing (proxy only) */}
      {!isRedirect && (
        <FormCard
          title="Path routing"
          desc="Send specific paths of this host to different backends, or block them. Applied most-specific-first (exact before prefix, longer before shorter); anything unmatched falls through to the upstreams above. The WAF runs once for the whole host, before the split."
        >
          <div className="space-y-3">
            {form.pathRules.map((rule, i) => {
              const update = (patch: Partial<PathRuleForm>) => setRules(form.pathRules.map((r, j) => (j === i ? { ...r, ...patch } : r)))
              const remove = () => setRules(form.pathRules.filter((_, j) => j !== i))
              return (
                <div key={i} className="border bg-muted/20 p-3 sm:p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <Input
                      className="min-w-[10rem] flex-1 font-mono"
                      placeholder="/api"
                      value={rule.path}
                      onChange={(e) => update({ path: e.target.value })}
                      aria-label="Path"
                    />
                    <Select value={rule.match} onValueChange={(v) => update({ match: v as "prefix" | "exact" })}>
                      <SelectTrigger className="w-32" aria-label="Match mode">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="prefix">prefix</SelectItem>
                        <SelectItem value="exact">exact</SelectItem>
                      </SelectContent>
                    </Select>
                    <Select value={rule.action} onValueChange={(v) => update({ action: v as "proxy" | "deny" })}>
                      <SelectTrigger className="w-32" aria-label="Action">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="proxy">proxy</SelectItem>
                        <SelectItem value="deny">deny · 403</SelectItem>
                      </SelectContent>
                    </Select>
                    <button
                      type="button"
                      onClick={remove}
                      className="grid size-8 shrink-0 place-items-center text-muted-foreground transition-colors hover:text-red-500"
                      aria-label="Remove path rule"
                    >
                      <X className="size-4" />
                    </button>
                  </div>
                  {rule.action === "proxy" && (
                    <div className="mt-3 grid gap-3 sm:grid-cols-2">
                      <div className="space-y-1">
                        <SubLabel>Upstreams</SubLabel>
                        <TokenInput ariaLabel="Path rule upstreams" value={rule.upstreams} onChange={(upstreams) => update({ upstreams })} placeholder="api-1.mesh:8000" />
                      </div>
                      <div className="space-y-1">
                        <SubLabel>Strip prefix</SubLabel>
                        <Input className="font-mono" placeholder="/api" value={rule.stripPrefix} onChange={(e) => update({ stripPrefix: e.target.value })} />
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
            <div className="flex items-center gap-3">
              <AddButton onClick={() => setRules([...form.pathRules, { path: "", match: "prefix", action: "proxy", upstreams: [], stripPrefix: "" }])}>
                Add path rule
              </AddButton>
              {form.pathRules.length === 0 && (
                <span className="text-xs text-muted-foreground">None yet — the whole host goes to the upstreams above.</span>
              )}
            </div>
          </div>
        </FormCard>
      )}

      {/* 6 · Advanced */}
      <FormCard
        title="Advanced"
        desc="A raw Caddyfile fragment for what the fields above can't express. Runs just before the proxy; validated on save — a syntax error is rejected."
      >
        <textarea
          value={form.rawCaddy}
          onChange={(e) => set({ rawCaddy: e.target.value })}
          rows={5}
          spellCheck={false}
          placeholder={"redir /old /new 302\n# the reverse_proxy is added automatically"}
          className="w-full resize-y rounded-none border bg-background px-3 py-2.5 font-mono text-xs leading-relaxed outline-none focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring/50"
        />
      </FormCard>
    </div>
  )
}
