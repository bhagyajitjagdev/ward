import { useLocation } from "@tanstack/react-router"
import { Moon, Sun } from "lucide-react"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { Separator } from "@/components/ui/separator"
import { Button } from "@/components/ui/button"
import { useQuery } from "@tanstack/react-query"
import { StatusDot } from "@/components/console"
import { useTheme } from "@/lib/theme"
import { api } from "@/lib/api"

const titles: Record<string, string> = {
  "/": "Overview",
  "/services": "Services",
  "/waf-events": "WAF Events",
  "/top-triggers": "Top Triggers",
  "/exclusions": "Exclusions",
  "/blocklist": "Blocklist",
  "/rate-limits": "Rate Limits",
  "/geo": "Geo Blocking",
  "/users": "Users",
  "/tokens": "API Tokens",
  "/audit": "Audit Log",
  "/settings": "Settings",
}

function titleFor(pathname: string): string {
  if (pathname === "/") return titles["/"]
  const key = Object.keys(titles).find((k) => k !== "/" && pathname.startsWith(k))
  return key ? titles[key] : "Ward"
}

export function AppHeader() {
  const { pathname } = useLocation()
  const { theme, toggle } = useTheme()
  const { data: overview } = useQuery({ queryKey: ["overview"], queryFn: api.overview })
  const { data: settings } = useQuery({ queryKey: ["settings"], queryFn: api.getSettings })
  const enforcing = settings?.waf_engine_mode === "On"

  return (
    <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-3 border-b bg-background/80 px-4 backdrop-blur-sm">
      <SidebarTrigger className="-ml-1 text-muted-foreground" />
      <Separator orientation="vertical" className="h-5" />
      <div className="flex items-center gap-2">
        <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">Ward</span>
        <span className="text-border">/</span>
        <span className="font-heading text-sm font-semibold">{titleFor(pathname)}</span>
      </div>

      <div className="ml-auto flex items-center gap-2">
        {overview && (
          <div className="hidden items-center gap-2.5 rounded-md border bg-muted/30 px-3 py-1.5 font-mono text-[11px] md:flex">
            <span className="flex items-center gap-1.5">
              <StatusDot tone={enforcing ? "threat" : "detecting"} /> {enforcing ? "Enforcing" : "DetectionOnly"}
            </span>
            <span className="text-border">·</span>
            <span className="text-muted-foreground">
              {overview.waf_services}/{overview.services} WAF
            </span>
            <span className="text-border">·</span>
            <span className="text-amber-500">{overview.detections_24h} det/24h</span>
          </div>
        )}
        <Button
          variant="ghost"
          size="icon"
          onClick={toggle}
          aria-label="Toggle theme"
          className="text-muted-foreground"
        >
          {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
        </Button>
      </div>
    </header>
  )
}
