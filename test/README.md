# Ward e2e / integration harness

`e2e.compose.yml` stands up the real edge — the custom **Caddy + Coraza** image (`../caddy`) — the
**Ward** control plane (`../backend`), a rich **appserver** upstream (echo / SSE / WebSocket / big
body), and a `traefik/whoami`. The **tester** (`e2e/`) then drives Ward's API and asserts behaviour
*through the edge*, all in-network — so the flaky host↔container port-forward is never used.

## Run

```bash
docker compose -p wardtest -f test/e2e.compose.yml up --build --exit-code-from tester --abort-on-container-exit
docker compose -p wardtest -f test/e2e.compose.yml down -v
```

The **tester's exit code is the result** (0 = every check passed). CI runs exactly this
(`.github/workflows/e2e.yml`) on every push and PR — including Renovate's dependency bumps — so a
Caddy/Coraza/CRS/module update that breaks the edge fails there before it ships.

## What it checks

`e2e/main.go` is a flat list of named checks (add one = add a function + a `check(...)` line):

- **proxy** · **multi-hostname** — routing, one route serves many hostnames
- **waf-enforce** — SQLi/XSS blocked (403), benign passes
- **waf-detect+crs-id** — detection-only passes but logs; asserts CRS rule **942100** still exists
  (guards against a CRS renumber silently breaking exclusions)
- **waf-exclusion** — a scoped exclusion silences a rule on one path, still active elsewhere
- **skip-paths-sse / -ws / -still-block** — SSE streams and WebSocket upgrades bypass the WAF, while
  normal paths on the same service still get blocked
- **ip-blocklist** · **rate-limit** — 403 then unblock→200; a burst trips the limit (429)
- **http-\*** — security-headers preset, add/remove header, strip-prefix, basic auth (401→200),
  compression
- **raw-caddyfile** — a `redir` fragment works; an invalid fragment is rejected at save
- **edge-versions** · **snapshots** — Settings reports the compiled-in versions; config snapshots exist

## Notes

- Ward reaches Caddy's admin at `http://caddy:2019`; Coraza writes its audit log to a shared volume
  Ward tails (`/shared/audit.log`). `bootstrap.json` is the minimal config Caddy boots with.
- The `appserver` / `tester` run via `go run` in a `golang` image over a read-only mount of `test/`
  (its own `go.mod`), so no extra Dockerfiles.
- `WARD_AUTO_HTTPS=1 docker compose ... up` enables auto-TLS (Caddy internal CA) for TLS testing.
- Ward runs as root here only so it can read Caddy's root-written audit log; a real deployment aligns
  users/perms instead.
