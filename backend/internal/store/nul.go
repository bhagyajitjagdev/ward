package store

import "strings"

// storable makes request-derived text writable. bun (since 1.3.0) rejects a string
// holding a NUL byte rather than silently stripping it — Postgres text can't hold
// one — and WAF/access events carry attacker-controlled request data (a decoded
// `%00` in a matched value). Unescaped, one such byte gets the event dropped at
// ingest, so a request could hide itself from the event log. The byte stays visible
// as a literal `\x00`.
func storable(s string) string {
	if strings.IndexByte(s, 0) < 0 {
		return s
	}
	return strings.ReplaceAll(s, "\x00", `\x00`)
}
