import { Link, useLocation, useNavigate } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import {
  Gauge,
  ShieldAlert,
  Flame,
  Activity,
  Server,
  FilterX,
  Ban,
  Timer,
  Globe,
  FileKey,
  Users,
  KeyRound,
  ScrollText,
  Settings,
  ShieldCheck,
  LogOut,
  ChevronsUpDown,
  ArrowUpCircle,
} from "lucide-react"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { StatusDot } from "@/components/console"
import { useLogout, useMe } from "@/lib/auth"
import { api } from "@/lib/api"
import { fetchLatestRelease, isNewerRelease, WARD_RELEASES_URL } from "@/lib/version"

const groups = [
  {
    label: "Monitor",
    items: [
      { title: "Overview", to: "/", icon: Gauge },
      { title: "WAF Events", to: "/waf-events", icon: ShieldAlert },
      { title: "Top Triggers", to: "/top-triggers", icon: Flame },
      { title: "Access Log", to: "/access", icon: Activity },
    ],
  },
  {
    label: "Edge",
    items: [
      { title: "Services", to: "/services", icon: Server },
      { title: "Exclusions", to: "/exclusions", icon: FilterX },
      { title: "Blocklist", to: "/blocklist", icon: Ban },
      { title: "Rate Limits", to: "/rate-limits", icon: Timer },
      { title: "Geo Blocking", to: "/geo", icon: Globe },
      { title: "Certificates", to: "/certificates", icon: FileKey },
    ],
  },
  {
    label: "Admin",
    items: [
      { title: "Users", to: "/users", icon: Users },
      { title: "API Tokens", to: "/tokens", icon: KeyRound },
      { title: "Audit Log", to: "/audit", icon: ScrollText },
    ],
  },
] as const

export function AppSidebar() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { data: user } = useMe()
  const logout = useLogout()

  const isActive = (to: string) => (to === "/" ? pathname === "/" : pathname.startsWith(to))
  const name = user?.username ?? "owner"
  const role = user?.is_owner ? "owner" : (user?.role ?? "admin")

  async function handleLogout() {
    await logout()
    await navigate({ to: "/login" })
  }

  return (
    <Sidebar>
      <SidebarHeader className="border-b">
        <div className="flex items-center gap-2.5 px-1 py-1.5">
          <div className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground shadow-sm">
            <ShieldCheck className="size-5" />
          </div>
          <div className="flex flex-col leading-none">
            <span className="font-heading text-base font-semibold">Ward</span>
            <span className="mt-1 flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
              <StatusDot tone="armed" pulse /> edge armed
            </span>
          </div>
        </div>
      </SidebarHeader>

      <SidebarContent>
        {groups.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel className="font-mono text-[10px] uppercase tracking-[0.18em]">
              {group.label}
            </SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton asChild isActive={isActive(item.to)} tooltip={item.title}>
                      <Link to={item.to}>
                        <item.icon />
                        <span>{item.title}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter className="border-t">
        <VersionBadge />
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton size="lg" className="gap-2">
                  <Avatar className="size-7 rounded-md">
                    <AvatarFallback className="rounded-md bg-muted text-xs font-medium">
                      {name.slice(0, 1).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <div className="flex flex-col items-start leading-none">
                    <span className="text-sm font-medium">{name}</span>
                    <span className="mt-0.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      {role}
                    </span>
                  </div>
                  <ChevronsUpDown className="ml-auto size-4 text-muted-foreground" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel className="font-normal">
                  <span className="text-muted-foreground">Signed in as </span>
                  {name}
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <Link to="/settings">
                    <Settings className="size-4" /> Settings
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={handleLogout}>
                  <LogOut className="size-4" /> Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  )
}

// VersionBadge shows the running Ward build and, when a newer GitHub release exists,
// an "Update available" link. The check runs in the browser (see @/lib/version): it
// works when the box is offline but the browser isn't, and never phones home from the
// edge. Offline / no release / rate-limited → just the version.
function VersionBadge() {
  const { data: v } = useQuery({ queryKey: ["ward-version"], queryFn: api.getVersion, staleTime: Infinity })
  const { data: latest } = useQuery({
    queryKey: ["ward-latest-release"],
    queryFn: fetchLatestRelease,
    staleTime: 60 * 60 * 1000, // an hour — don't hammer GitHub
    retry: false,
  })
  const current = v?.version
  const update = isNewerRelease(latest, current)

  if (update) {
    return (
      <a
        href={WARD_RELEASES_URL}
        target="_blank"
        rel="noreferrer"
        className="mx-1 flex items-center justify-between gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-2.5 py-1.5 text-amber-600 transition-colors hover:bg-amber-500/20 dark:text-amber-400"
        title={`Ward ${current} → ${latest}`}
      >
        <span className="flex items-center gap-1.5 font-mono text-[10px] font-medium uppercase tracking-wider">
          <ArrowUpCircle className="size-3.5" /> Update available
        </span>
        <span className="font-mono text-[10px] text-amber-600/70 dark:text-amber-400/70">{latest}</span>
      </a>
    )
  }
  return (
    <div className="mx-1 flex items-center gap-1.5 px-1.5 py-1 font-mono text-[10px] uppercase tracking-wider text-muted-foreground/60">
      <span>Ward</span>
      <span className="normal-case tracking-normal">{current ?? "—"}</span>
    </div>
  )
}
