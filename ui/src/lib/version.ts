// Client-side update check. It runs in the browser (not on the box), so it works
// even when the edge is airgapped but the operator's browser has internet — and the
// control plane never phones home. Any failure (offline / no releases yet / GitHub
// rate-limit) resolves to null, and the UI just shows the current version, no nag.
export const WARD_REPO = "bhagyajitjagdev/ward"
export const WARD_RELEASES_URL = `https://github.com/${WARD_REPO}/releases`

export async function fetchLatestRelease(): Promise<string | null> {
  try {
    const res = await fetch(`https://api.github.com/repos/${WARD_REPO}/releases/latest`, {
      headers: { Accept: "application/vnd.github+json" },
    })
    if (!res.ok) return null
    const data = (await res.json()) as { tag_name?: unknown }
    return typeof data.tag_name === "string" ? data.tag_name : null
  } catch {
    return null
  }
}

// A clean release tag like v1.2.3 (no -dev / -gSHA suffix, not a bare commit hash).
// Dev builds never trigger the update nag — they just show their version string.
function isCleanRelease(v: string): boolean {
  return /^v?\d+\.\d+\.\d+$/.test(v.trim())
}

function parts(v: string): number[] {
  return v
    .trim()
    .replace(/^v/, "")
    .split(".")
    .map((n) => parseInt(n, 10) || 0)
}

// isNewerRelease reports whether `latest` is a strictly newer release than `current`
// (numeric semver compare). False unless both are clean release tags.
export function isNewerRelease(latest: string | null | undefined, current: string | null | undefined): boolean {
  if (!latest || !current || !isCleanRelease(latest) || !isCleanRelease(current)) return false
  const a = parts(latest)
  const b = parts(current)
  for (let i = 0; i < 3; i++) {
    if ((a[i] || 0) !== (b[i] || 0)) return (a[i] || 0) > (b[i] || 0)
  }
  return false
}
