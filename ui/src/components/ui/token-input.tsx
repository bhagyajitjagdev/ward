import { useState, type KeyboardEvent } from "react"
import { X } from "lucide-react"
import { cn } from "@/lib/utils"

// TokenInput is a controlled multi-value field: each value is a removable chip,
// typed text is committed on Enter / comma / blur (and paste with commas/spaces),
// Backspace on an empty input removes the last chip. Replaces comma-separated text.
export function TokenInput({
  value,
  onChange,
  placeholder,
  validate,
  id,
  ariaLabel,
  className,
}: {
  value: string[]
  onChange: (v: string[]) => void
  placeholder?: string
  validate?: (v: string) => string | null // return an error string to reject a token
  id?: string
  ariaLabel?: string
  className?: string
}) {
  const [draft, setDraft] = useState("")
  const [error, setError] = useState<string | null>(null)

  const commit = (raw: string) => {
    const parts = raw
      .split(/[\s,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    if (parts.length === 0) return
    const next = value.slice()
    for (const p of parts) {
      if (validate) {
        const err = validate(p)
        if (err) {
          setError(err)
          return
        }
      }
      if (!next.includes(p)) next.push(p)
    }
    onChange(next)
    setDraft("")
    setError(null)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault()
      commit(draft)
    } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  return (
    <div className="space-y-1">
      <div
        className={cn(
          "flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border bg-background px-2 py-1.5 shadow-xs focus-within:ring-2 focus-within:ring-ring/50",
          className,
        )}
      >
        {value.map((v) => (
          <span
            key={v}
            className="inline-flex items-center gap-1 rounded border bg-muted/60 px-1.5 py-0.5 font-mono text-xs"
          >
            {v}
            <button
              type="button"
              onClick={() => onChange(value.filter((x) => x !== v))}
              aria-label={`Remove ${v}`}
              className="text-muted-foreground transition-colors hover:text-red-500"
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        <input
          id={id}
          aria-label={ariaLabel}
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value)
            setError(null)
          }}
          onKeyDown={onKeyDown}
          onBlur={() => commit(draft)}
          placeholder={value.length === 0 ? placeholder : ""}
          className="min-w-[8rem] flex-1 bg-transparent font-mono text-sm outline-none placeholder:font-sans placeholder:text-muted-foreground"
        />
      </div>
      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}
