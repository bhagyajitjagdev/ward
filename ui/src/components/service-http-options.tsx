import { useState } from "react"
import { Plus, X, ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"
import type { HTTPConfig } from "@/lib/api"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { TokenInput } from "@/components/ui/token-input"

// HeaderEditor edits a header map as add/removable key/value rows. Rows are keyed by
// position (not header name) so typing a key never remounts/steals focus.
function HeaderEditor({
  label,
  value,
  onChange,
}: {
  label: string
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
      <Label className="text-xs text-muted-foreground">{label}</Label>
      {rows.map((r, i) => (
        <div key={i} className="flex items-center gap-1.5">
          <Input
            className="h-8 font-mono text-xs"
            placeholder="Header"
            value={r.k}
            onChange={(e) => sync(rows.map((x, j) => (j === i ? { ...x, k: e.target.value } : x)))}
          />
          <Input
            className="h-8 font-mono text-xs"
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
            <X className="size-3.5" />
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

// ServiceHttpOptions is the collapsible per-service HTTP controls: security-headers
// preset, basic auth, request/response headers, header removal, path strip, compression.
export function ServiceHttpOptions({
  value,
  onChange,
  rawCaddy,
  onRawChange,
  editing,
}: {
  value: HTTPConfig
  onChange: (v: HTTPConfig) => void
  rawCaddy: string
  onRawChange: (v: string) => void
  editing?: boolean
}) {
  const [open, setOpen] = useState(false)
  const v = value ?? {}
  const set = (patch: Partial<HTTPConfig>) => onChange({ ...v, ...patch })
  return (
    <div className="rounded-lg border">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 px-3 py-2 text-sm font-medium"
      >
        <ChevronRight className={cn("size-4 transition-transform", open && "rotate-90")} />
        HTTP options <span className="font-normal text-muted-foreground">— headers, auth, path, compression</span>
      </button>
      {open && (
        <div className="space-y-4 border-t p-3">
          <label className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              className="mt-0.5"
              checked={!!v.security_headers}
              onChange={(e) => set({ security_headers: e.target.checked })}
            />
            <span>
              Security headers{" "}
              <span className="text-xs text-muted-foreground">— HSTS, X-Frame-Options, nosniff, Referrer-Policy</span>
            </span>
          </label>

          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Basic auth</Label>
            <div className="grid grid-cols-2 gap-1.5">
              <Input
                className="h-8"
                placeholder="username"
                value={v.basic_auth_user ?? ""}
                onChange={(e) => set({ basic_auth_user: e.target.value })}
              />
              <Input
                className="h-8"
                type="password"
                placeholder={editing ? "leave blank to keep" : "password"}
                value={v.basic_auth_password ?? ""}
                onChange={(e) => set({ basic_auth_password: e.target.value })}
              />
            </div>
            <p className="text-[11px] text-muted-foreground">Clear the username to turn auth off.</p>
          </div>

          <HeaderEditor
            label="Response headers (to client)"
            value={v.response_headers}
            onChange={(h) => set({ response_headers: h })}
          />
          <HeaderEditor
            label="Request headers (to upstream)"
            value={v.request_headers}
            onChange={(h) => set({ request_headers: h })}
          />

          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground">Strip response headers</Label>
            <TokenInput
              value={v.remove_headers ?? []}
              onChange={(r) => set({ remove_headers: r })}
              placeholder="Server"
              ariaLabel="Strip response headers"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="strip-prefix" className="text-xs text-muted-foreground">
              Strip path prefix
            </Label>
            <Input
              id="strip-prefix"
              className="h-8 font-mono"
              placeholder="/api"
              value={v.strip_path_prefix ?? ""}
              onChange={(e) => set({ strip_path_prefix: e.target.value })}
            />
          </div>

          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={!!v.compression}
              onChange={(e) => set({ compression: e.target.checked })}
            />
            <span>
              Compression <span className="text-xs text-muted-foreground">— gzip / zstd</span>
            </span>
          </label>

          <div className="space-y-1.5 border-t pt-3">
            <Label htmlFor="raw-caddy" className="text-xs text-muted-foreground">
              Advanced — raw Caddyfile <span className="text-[11px]">(for what the fields above can't do)</span>
            </Label>
            <textarea
              id="raw-caddy"
              value={rawCaddy}
              onChange={(e) => onRawChange(e.target.value)}
              rows={4}
              spellCheck={false}
              placeholder={"redir /old /new 302\nreverse_proxy is added automatically"}
              className="w-full resize-y rounded-md border bg-background px-3 py-2 font-mono text-xs leading-relaxed shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
            />
            <p className="text-[11px] text-muted-foreground">
              Directives run just before the proxy. Validated against the edge on save — a syntax error is rejected.
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
