# Contributing to Ward

Thanks for your interest! Ward is a personal open-source project — issues, ideas, and pull requests
are all welcome.

## Dev setup

- **Backend** — Go (module `github.com/bhagyajitjagdev/ward/backend`), [bun](https://bun.uptrace.dev)
  + pure-Go SQLite (`modernc`, no CGO), [goose](https://github.com/pressly/goose) migrations
  (embedded, dual-dialect).
  - Build: `go -C backend build ./...`
  - Run:   `go -C backend run ./cmd/ward`  — serves the API under `/api` on `:8080`
  - Test:  `go -C backend test ./...`
  - Key env: `WARD_DB` (a SQLite file, or a `postgres://…` DSN), `WARD_ADDR` (default `:8080`),
    `WARD_CADDY_ADMIN` (default `http://localhost:2019`).
- **Frontend** — `ui/` (Vite + React + TanStack Router + shadcn).
  - Dev:   `npm --prefix ui run dev`  — `:5173`, proxies `/api` → the backend
  - Build: `npm --prefix ui run build`
- **Edge image** — `caddy/` (custom `xcaddy`: Coraza + rate-limit + maxmind-geolocation).
- **Full stack** — `deploy/docker-compose.yml`, or the end-to-end harness in `test/`.

## Repo layout

```
backend/   Go control plane (API + embedded UI)
ui/        ward-ui — Vite + React + TanStack Router
caddy/     custom xcaddy edge image (Caddy + Coraza + modules)
deploy/    docker-compose stack + example env
test/      e2e / integration harness
```

## Principles (please preserve)

1. **Control plane, not data plane** — Ward configures Caddy and reads its logs; it is never in the
   request path. The edge must keep serving even if Ward is down.
2. **The DB is the single source of truth** — the Caddy config is *derived*: regenerate → validate →
   snapshot → rollback. Ward is the only writer of Caddy config.
3. **Integrate, don't rebuild.**
4. **API-first** — sessions for humans, revocable machine tokens for scripts.

## Pull requests

- **Verify by running.** Drive the actual flow end-to-end — anything touching the edge should be
  checked against the real Caddy image, not only unit tests.
- Keep the Caddy admin API (`:2019`) internal-only and the management plane private.
- Render untrusted log/event data as **text** (never `dangerouslySetInnerHTML`).
- Keep `go vet` and `tsc` clean.

By contributing you agree your work is licensed under the project's [MIT license](LICENSE).
