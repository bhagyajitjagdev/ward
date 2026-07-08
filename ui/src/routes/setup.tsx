import { useState } from "react"
import { createFileRoute, useNavigate, Link, redirect } from "@tanstack/react-router"
import { toast } from "sonner"
import { api, ApiError, getToken, setToken } from "@/lib/api"
import { AuthShell } from "@/components/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export const Route = createFileRoute("/setup")({
  beforeLoad: () => {
    if (getToken()) throw redirect({ to: "/" })
  },
  component: SetupPage,
})

function SetupPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password.length < 8) {
      toast.error("Password must be at least 8 characters")
      return
    }
    setLoading(true)
    try {
      const res = await api.setup(username, password)
      setToken(res.token)
      toast.success("Owner account created")
      await navigate({ to: "/" })
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        toast.error("Ward is already set up — please sign in")
        await navigate({ to: "/login" })
        return
      }
      toast.error(err instanceof Error ? err.message : "Setup failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell>
      <div className="space-y-6">
        <div className="space-y-1.5">
          <div className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground">First run</div>
          <h2 className="font-heading text-2xl font-semibold tracking-tight">Create the owner account</h2>
          <p className="text-sm text-muted-foreground">
            This is the first and only account that can add others. Choose it carefully.
          </p>
        </div>
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">At least 8 characters.</p>
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Creating…" : "Create owner account"}
          </Button>
        </form>
        <p className="text-sm text-muted-foreground">
          Already set up?{" "}
          <Link to="/login" className="font-medium text-primary underline-offset-4 hover:underline">
            Sign in
          </Link>
        </p>
      </div>
    </AuthShell>
  )
}
