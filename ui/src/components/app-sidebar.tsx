import { Link, useLocation, useNavigate } from "@tanstack/react-router"
import {
  Gauge,
  ShieldAlert,
  Flame,
  Server,
  FilterX,
  Ban,
  Users,
  KeyRound,
  ScrollText,
  Settings,
  ShieldCheck,
  LogOut,
  ChevronsUpDown,
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

const groups = [
  {
    label: "Monitor",
    items: [
      { title: "Overview", to: "/", icon: Gauge },
      { title: "WAF Events", to: "/waf-events", icon: ShieldAlert },
      { title: "Top Triggers", to: "/top-triggers", icon: Flame },
    ],
  },
  {
    label: "Edge",
    items: [
      { title: "Services", to: "/services", icon: Server },
      { title: "Exclusions", to: "/exclusions", icon: FilterX },
      { title: "Blocklist", to: "/blocklist", icon: Ban },
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
