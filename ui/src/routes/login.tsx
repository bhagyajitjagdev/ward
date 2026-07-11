import { useState } from "react"
import { createFileRoute, useNavigate, Link, redirect } from "@tanstack/react-router"
import { toast } from "sonner"
import { api, getToken, setToken } from "@/lib/api"
import { AuthShell } from "@/components/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export const Route = createFileRoute("/login")({
  beforeLoad: async () => {
    if (getToken()) throw redirect({ to: "/" })
    // First run (no owner yet) → send them to setup instead of a dead login form.
    if ((await api.setupState()).needs_setup) throw redirect({ to: "/setup" })
  },
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await api.login(username, password)
      setToken(res.token)
      await navigate({ to: "/" })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Login failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <AuthShell>
      <div className="space-y-6">
        <div className="space-y-1.5">
          <h2 className="font-heading text-2xl font-semibold tracking-tight">Sign in</h2>
          <p className="text-sm text-muted-foreground">Manage your security edge.</p>
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
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Signing in…" : "Sign in"}
          </Button>
        </form>
        <p className="text-sm text-muted-foreground">
          First time?{" "}
          <Link to="/setup" className="font-medium text-primary underline-offset-4 hover:underline">
            Create the owner account
          </Link>
        </p>
      </div>
    </AuthShell>
  )
}
