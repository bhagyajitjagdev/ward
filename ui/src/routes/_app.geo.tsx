import { useRef, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Globe, Trash2, AlertTriangle, Upload, Download, KeyRound } from "lucide-react"
import { PageHeader, StatusDot, Mono, ago, ModeBadge, ModeToggle } from "@/components/console"
import type { RuleMode } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { GeoRule } from "@/lib/api"
import { useServices, useServiceNames } from "@/data/queries"
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

export const Route = createFileRoute("/_app/geo")({
  component: GeoPage,
})

function fmtBytes(n?: number): string {
  if (!n) return "0 B"
  const mb = n / (1024 * 1024)
  return mb >= 1 ? `${mb.toFixed(1)} MB` : `${(n / 1024).toFixed(0)} KB`
}

function GeoPage() {
  return (
    <div className="space-y-8">
      <PageHeader
        eyebrow="Edge"
        title="Geo Blocking"
        description="Block or allow traffic by country — globally or per service. Needs a GeoIP database, which you can bring however you like."
        actions={<AddRuleDialog />}
      />
      <GeoIPPanel />
      <RulesTable />
    </div>
  )
}

function GeoIPPanel() {
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const { data: st, isLoading } = useQuery({ queryKey: ["geoip"], queryFn: api.geoipStatus })

  const done = (msg: string) => {
    qc.invalidateQueries({ queryKey: ["geoip"] })
    toast.success(msg)
  }
  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : "Something went wrong")

  const dbip = useMutation({ mutationFn: api.geoipDBIP, onSuccess: () => done("DB-IP Lite database installed"), onError: fail })
  const upload = useMutation({ mutationFn: (f: File) => api.geoipUpload(f), onSuccess: () => done("Database uploaded"), onError: fail })
  const del = useMutation({ mutationFn: api.geoipDelete, onSuccess: () => done("GeoIP database removed"), onError: fail })

  return (
    <section className="rounded-xl border bg-card">
      <div className="flex items-baseline gap-2 border-b px-5 py-3.5">
        <h2 className="font-heading text-sm font-semibold">GeoIP database</h2>
        <span className="font-mono text-[11px] text-muted-foreground">the IP → country data the edge reads</span>
      </div>
      <div className="space-y-4 p-5">
        {isLoading ? (
          <Skeleton className="h-14 w-full" />
        ) : st?.present ? (
          <div className="flex items-center gap-3 rounded-lg border bg-muted/30 p-3">
            <StatusDot tone="ok" />
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium">{st.filename}</div>
              <Mono dim className="block truncate !text-xs">
                {st.source} · {fmtBytes(st.size)} · updated {st.updated_at ? ago(st.updated_at) : "—"}
              </Mono>
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="text-muted-foreground hover:text-red-500"
              disabled={del.isPending}
              onClick={() => del.mutate()}
            >
              <Trash2 className="size-4" /> Remove
            </Button>
          </div>
        ) : (
          <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-600 dark:text-amber-400">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            No database installed — the rules below won't take effect until you add one.
          </div>
        )}

        <div className="space-y-2">
          <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            {st?.present ? "Replace / update" : "Add a database"}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" size="sm" onClick={() => dbip.mutate()} disabled={dbip.isPending}>
              <Download className="size-4" /> {dbip.isPending ? "Downloading…" : "DB-IP Lite (free)"}
            </Button>
            <Button variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={upload.isPending}>
              <Upload className="size-4" /> {upload.isPending ? "Uploading…" : "Upload .mmdb"}
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept=".mmdb"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0]
                if (f) upload.mutate(f)
                e.target.value = ""
              }}
            />
            <MaxMindDialog onDone={() => done("GeoLite2 database installed")} />
          </div>
          <p className="text-xs text-muted-foreground">
            Or drop any <Mono>.mmdb</Mono> into <Mono>{st?.dir ?? "/geoip"}</Mono> (a mounted volume) and Ward picks it
            up automatically.
          </p>
        </div>
      </div>
    </section>
  )
}

function MaxMindDialog({ onDone }: { onDone: () => void }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const fetch = useMutation({
    mutationFn: () => api.geoipMaxMind(key.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["geoip"] })
      onDone()
      setOpen(false)
      setKey("")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "MaxMind download failed"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <KeyRound className="size-4" /> MaxMind
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>MaxMind GeoLite2</DialogTitle>
          <DialogDescription>
            Ward downloads GeoLite2-Country with your license key and stores it in the GeoIP volume. Get a free key from
            your MaxMind account.
          </DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            fetch.mutate()
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="mm-key">License key</Label>
            <Input id="mm-key" className="font-mono" value={key} onChange={(e) => setKey(e.target.value)} placeholder="XXXXXX_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!key.trim() || fetch.isPending}>
              {fetch.isPending ? "Downloading…" : "Download"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function RulesTable() {
  const qc = useQueryClient()
  const names = useServiceNames()
  const { data: rules, isLoading, error } = useQuery({ queryKey: ["geo-rules"], queryFn: api.listGeoRules })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteGeoRule(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["geo-rules"] })
      toast.success("Geo rule removed")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove the rule"),
  })

  return (
    <div className="overflow-hidden rounded-xl border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="px-4 py-2.5 font-medium">Scope</th>
            <th className="px-4 py-2.5 font-medium">Countries</th>
            <th className="px-4 py-2.5 font-medium">Added</th>
            <th className="w-10" />
          </tr>
        </thead>
        <tbody className="divide-y">
          {isLoading && (
            <tr>
              <td colSpan={4} className="px-4 py-3.5">
                <Skeleton className="h-6 w-full" />
              </td>
            </tr>
          )}
          {error && (
            <tr>
              <td colSpan={4} className="py-12 text-center text-sm text-red-500">
                Couldn't load geo rules.
              </td>
            </tr>
          )}
          {rules?.length === 0 && (
            <tr>
              <td colSpan={4} className="py-16 text-center text-sm text-muted-foreground">
                No geo rules yet — block a set of countries, or allow only some.
              </td>
            </tr>
          )}
          {rules?.map((g) => (
            <tr key={g.id} className="group transition-colors hover:bg-muted/40">
              <td className="px-4 py-3">
                <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                  {g.scope === "service" ? (g.service_id ? (names[g.service_id] ?? "service") : "service") : "global"}
                </span>
              </td>
              <td className="px-4 py-3">
                <div className="flex flex-wrap items-center gap-1.5">
                  <ModeBadge mode={g.mode} />
                  {g.countries.map((c) => (
                    <span
                      key={c}
                      className={`rounded border px-1.5 py-0.5 font-mono text-[11px] ${
                        g.mode === "allow"
                          ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                          : "border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400"
                      }`}
                    >
                      {c}
                    </span>
                  ))}
                </div>
              </td>
              <td className="px-4 py-3">
                <Mono dim className="!text-xs">
                  {ago(g.created_at)}
                </Mono>
              </td>
              <td className="pr-3">
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                  aria-label="Remove rule"
                  disabled={remove.isPending}
                  onClick={() => remove.mutate(g.id)}
                >
                  <Trash2 className="size-4" />
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function AddRuleDialog() {
  const qc = useQueryClient()
  const { data: services } = useServices()
  const [open, setOpen] = useState(false)
  const [scope, setScope] = useState("global") // "global" | <service id>
  const [countries, setCountries] = useState("")
  const [mode, setMode] = useState<RuleMode>("block")

  const create = useMutation({
    mutationFn: () =>
      api.createGeoRule({
        mode,
        scope: scope === "global" ? "global" : "service",
        service_id: scope === "global" ? undefined : scope,
        countries: countries
          .split(",")
          .map((c) => c.trim())
          .filter(Boolean),
      }),
    onSuccess: (g: GeoRule) => {
      qc.invalidateQueries({ queryKey: ["geo-rules"] })
      toast.success(
        g.mode === "allow" ? `Allowing only ${g.countries.join(", ")}` : `Blocking ${g.countries.join(", ")}`,
      )
      setOpen(false)
      setScope("global")
      setCountries("")
      setMode("block")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't add the rule"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <Globe className="size-4" /> Add rule
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add a geo rule</DialogTitle>
          <DialogDescription>A 403 at the edge, by country, before requests reach the service.</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate()
          }}
          className="space-y-4"
        >
          <ModeToggle value={mode} onChange={setMode} />
          {mode === "allow" && (
            <div className="flex items-start gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-600 dark:text-amber-400">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <span>
                Allow-only 403s <strong>every</strong> country except these
                {scope === "global" ? " across the whole edge" : " for this service"} — including your own. List every
                country that should have access.
              </span>
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="scope">Scope</Label>
            <Select value={scope} onValueChange={setScope}>
              <SelectTrigger id="scope" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="global">Global · every service</SelectItem>
                {services?.map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor="countries">Countries</Label>
            <Input
              id="countries"
              className="font-mono uppercase"
              value={countries}
              onChange={(e) => setCountries(e.target.value)}
              placeholder="RU, CN, KP"
            />
            <p className="text-xs text-muted-foreground">Two-letter ISO codes, comma-separated.</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Adding…" : mode === "allow" ? "Allow only these" : "Block"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
