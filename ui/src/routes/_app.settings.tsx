import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { toast } from "sonner"
import { AlertTriangle, Moon, Sun } from "lucide-react"
import { cn } from "@/lib/utils"
import { PageHeader } from "@/components/console"
import { useTheme } from "@/lib/theme"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

export const Route = createFileRoute("/_app/settings")({
  component: SettingsPage,
})

function SettingsPage() {
  const { theme, toggle } = useTheme()
  const [mode, setMode] = useState<"DetectionOnly" | "Blocking">("DetectionOnly")
  const [email, setEmail] = useState("ops@acme.com")
  const [retention, setRetention] = useState("30")

  return (
    <div className="space-y-6">
      <PageHeader eyebrow="Admin" title="Settings" description="Edge-wide configuration. Changes are recorded in the audit log." />

      <div className="max-w-3xl space-y-6">
        <Section
          title="WAF engine"
          description="How the WAF treats detections across every protected service."
        >
          <Segmented
            options={[
              { value: "DetectionOnly", label: "Detection only" },
              { value: "Blocking", label: "Blocking" },
            ]}
            value={mode}
            onChange={(v) => setMode(v as typeof mode)}
          />
          {mode === "Blocking" ? (
            <p className="flex items-start gap-2 text-xs text-amber-500">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
              Attacks will be answered with 403. Make sure you've tuned false positives first — check Top Triggers.
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              The WAF logs detections but lets requests through. Safe for soaking before you flip to blocking.
            </p>
          )}
        </Section>

        <Section
          title="TLS"
          description="ACME account email for managed (Let's Encrypt) certificates."
          onSave={() => toast.success("TLS settings saved")}
        >
          <Input
            className="max-w-sm font-mono"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="ops@acme.com"
          />
        </Section>

        <Section
          title="Log retention"
          description="How long WAF events are kept before Ward prunes them."
          onSave={() => toast.success("Retention saved")}
        >
          <Select value={retention} onValueChange={setRetention}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="7">7 days</SelectItem>
              <SelectItem value="30">30 days</SelectItem>
              <SelectItem value="90">90 days</SelectItem>
              <SelectItem value="365">1 year</SelectItem>
            </SelectContent>
          </Select>
        </Section>

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
