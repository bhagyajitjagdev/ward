import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { KeyRound, Trash2, Copy, Check } from "lucide-react"
import { PageHeader, Mono, ago } from "@/components/console"
import { api, ApiError } from "@/lib/api"
import type { ApiToken } from "@/lib/api"
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

export const Route = createFileRoute("/_app/tokens")({
  component: ApiTokensPage,
})

function ApiTokensPage() {
  const qc = useQueryClient()
  const { data: tokens, isLoading, error } = useQuery({ queryKey: ["api-tokens"], queryFn: api.listApiTokens })
  const revoke = useMutation({
    mutationFn: (id: string) => api.revokeApiToken(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-tokens"] })
      toast.success("Token revoked")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't revoke token"),
  })

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Admin"
        title="API Tokens"
        description="Bearer tokens for scripts, CI, and Terraform. Shown once when created, hashed at rest, revocable anytime."
        actions={<CreateTokenDialog />}
      />

      <div className="overflow-hidden rounded-xl border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/30 text-left font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="px-4 py-2.5 font-medium">Name</th>
              <th className="px-4 py-2.5 font-medium">Created</th>
              <th className="px-4 py-2.5 font-medium">Last used</th>
              <th className="px-4 py-2.5 font-medium">Status</th>
              <th className="w-10" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {isLoading && (
              <tr>
                <td colSpan={5} className="px-4 py-3.5">
                  <Skeleton className="h-6 w-full" />
                </td>
              </tr>
            )}
            {error && (
              <tr>
                <td colSpan={5} className="py-12 text-center text-sm text-red-500">
                  Couldn't load tokens.
                </td>
              </tr>
            )}
            {tokens?.length === 0 && (
              <tr>
                <td colSpan={5} className="py-16 text-center text-sm text-muted-foreground">
                  No tokens yet — create one for a script or CI job.
                </td>
              </tr>
            )}
            {tokens?.map((t) => (
              <tr key={t.id} className="group transition-colors hover:bg-muted/40">
                <td className="px-4 py-3 font-medium">{t.name}</td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {ago(t.created_at)}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  <Mono dim className="!text-xs">
                    {t.last_used_at ? ago(t.last_used_at) : "never"}
                  </Mono>
                </td>
                <td className="px-4 py-3">
                  {t.revoked ? (
                    <span className="inline-flex items-center rounded border bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                      revoked
                    </span>
                  ) : (
                    <span className="inline-flex items-center rounded border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[11px] text-emerald-600 dark:text-emerald-400">
                      active
                    </span>
                  )}
                </td>
                <td className="pr-3">
                  {!t.revoked && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground opacity-0 transition-opacity hover:text-red-500 group-hover:opacity-100"
                      aria-label="Revoke token"
                      disabled={revoke.isPending}
                      onClick={() => revoke.mutate(t.id)}
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

function CreateTokenDialog() {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [issued, setIssued] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  function reset() {
    setName("")
    setIssued(null)
    setCopied(false)
  }

  const create = useMutation({
    mutationFn: () => api.createApiToken(name.trim()),
    onSuccess: (tok: ApiToken) => {
      qc.invalidateQueries({ queryKey: ["api-tokens"] })
      setIssued(tok.token ?? "")
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "Couldn't create token"),
  })

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (!o) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button>
          <KeyRound className="size-4" /> Create token
        </Button>
      </DialogTrigger>
      <DialogContent>
        {!issued ? (
          <>
            <DialogHeader>
              <DialogTitle>Create API token</DialogTitle>
              <DialogDescription>Give it a name you'll recognize. You'll see the token once.</DialogDescription>
            </DialogHeader>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                create.mutate()
              }}
              className="space-y-4"
            >
              <div className="space-y-2">
                <Label htmlFor="token-name">Name</Label>
                <Input
                  id="token-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="ci-github"
                  autoFocus
                />
              </div>
              <DialogFooter>
                <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
                  Cancel
                </Button>
                <Button type="submit" disabled={!name.trim() || create.isPending}>
                  {create.isPending ? "Creating…" : "Create token"}
                </Button>
              </DialogFooter>
            </form>
          </>
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>Copy your token now</DialogTitle>
              <DialogDescription>
                This is the only time it's shown. Store it somewhere safe — Ward keeps only a hash.
              </DialogDescription>
            </DialogHeader>
            <div className="flex items-center gap-2 rounded-lg border bg-muted/40 p-3">
              <Mono className="min-w-0 flex-1 truncate !text-xs">{issued}</Mono>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  navigator.clipboard?.writeText(issued)
                  setCopied(true)
                  toast.success("Token copied")
                }}
              >
                {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <DialogFooter>
              <Button onClick={() => setOpen(false)}>Done</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
