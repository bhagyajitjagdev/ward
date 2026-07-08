import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { UserPlus, Trash2 } from "lucide-react"
import { PageHeader, Mono, ago } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
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

export const Route = createFileRoute("/_app/users")({
  component: UsersPage,
})

function UsersPage() {
  const qc = useQueryClient()
  const { data: users, isLoading, error } = useQuery({ queryKey: ["users"], queryFn: api.listUsers })
  const remove = useMutation({
    mutationFn: (id: string) => api.deleteUser(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users"] })
      toast.success("User removed")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't remove user"),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="Users"
        description="People who can manage this edge. Every action they take is recorded in the audit log."
        actions={<InviteDialog />}
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">User</th>
              <th className="px-4 py-2.5 font-medium">Role</th>
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
                  Couldn't load users.
                </td>
              </tr>
            )}
            {users?.map((u) => (
              <tr key={u.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3">
                  <div className="flex items-center gap-3">
                    <Avatar className="size-8 rounded-md">
                      <AvatarFallback className="rounded-md bg-muted text-xs font-medium">
                        {u.username.slice(0, 1).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>
                    <span className="font-medium">{u.username}</span>
                  </div>
                </td>
                <td className="px-4 py-3">
                  {u.is_owner ? (
                    <span className="inline-flex items-center rounded border border-primary/30 bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] text-primary">
                      owner
                    </span>
                  ) : (
                    <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                      {u.role}
                    </span>
                  )}
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(u.created_at)}
                  </Mono>
                </td>
                <td className="pr-3">
                  {!u.is_owner && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                      aria-label="Remove user"
                      disabled={remove.isPending}
                      onClick={() => remove.mutate(u.id)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function InviteDialog() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")

  const create = useMutation({
    mutationFn: () => api.createUser(username.trim(), password),
    onSuccess: (u) => {
      qc.invalidateQueries({ queryKey: ["users"] })
      toast.success(`Added ${u.username}`)
      setOpen(false)
      setUsername("")
      setPassword("")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't add user"),
  })

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>
          <UserPlus className="size-4" /> Add user
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add user</DialogTitle>
          <DialogDescription>Set their username and an initial password. Everyone is an admin for now.</DialogDescription>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (password.length < 8) {
              toast.error("Password must be at least 8 characters")
              return
            }
            create.mutate()
          }}
          className="space-y-4"
        >
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="alice" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Initial password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">At least 8 characters.</p>
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? "Adding…" : "Add user"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
