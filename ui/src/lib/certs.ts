import type { Certificate } from "./api"

// sanMatches mirrors the backend certs.SANMatches: a subject (CN/SAN) secures host,
// honoring a single-label wildcard (*.example.com → a.example.com, but not b.a.example.com).
// A custom cert is keyed on disk by one folder name, but it serves every host in its SAN —
// so linking a service to "its" cert must match by SAN, not by the storage-folder name.
export function sanMatches(host: string, san: string): boolean {
  host = host.toLowerCase().trim()
  san = san.toLowerCase().trim()
  if (!host || !san) return false
  if (san === host) return true
  if (san.startsWith("*.")) {
    const suffix = san.slice(1) // ".example.com"
    if (host.endsWith(suffix)) {
      const label = host.slice(0, host.length - suffix.length)
      return label.length > 0 && !label.includes(".")
    }
  }
  return false
}

// certForHost returns the uploaded cert whose SAN covers host — the one Caddy actually
// serves for a "custom" service — or undefined if none is uploaded yet.
export function certForHost(certs: Certificate[] | undefined, host: string): Certificate | undefined {
  return (certs ?? []).find((c) => (c.subjects ?? []).some((san) => sanMatches(host, san)))
}
