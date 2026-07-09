# Ward

**A self-hosted, open-source security edge** — a humane control plane (web UI + API) over
[Caddy](https://caddyserver.com) + [Coraza](https://coraza.io) that runs a reverse proxy + WAF and
makes it *actually manageable*: validate-before-apply, one-click rollback, and a **WAF tuning
assistant** that turns "grep logs for hours" into "here's the exact scoped exclusion — apply it?".

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Images](https://github.com/bhagyajitjagdev/ward/actions/workflows/release.yml/badge.svg)](https://github.com/bhagyajitjagdev/ward/actions/workflows/release.yml)

> **Status:** 🟢 early implementation, dogfood-first. The core is feature-complete and verified
> against real Caddy + Coraza — not yet 1.0.

## Why

A WAF in front of your services is easy to switch on and miserable to operate. You run it in
detection-only mode "for now", drown in false positives, never quite trust it enough to flip to
blocking — so it just watches. Ward makes the tune-then-enforce loop a first-class, one-click
workflow: see a detection, judge it, apply the *exact* scoped exclusion Ward generates, and only
then turn on enforcement — globally or one service at a time.

## Features

- **Reverse proxy** — add a service (hostname → upstreams), load-balancing policies, auto-TLS.
- **WAF** (Coraza + OWASP CRS) — detection-only by default; flip to **enforcing** globally or
  **per service** once it's tuned.
- **Tuning assistant (the wedge)** — searchable detections + top triggers → one-click scoped
  exclusion, applied live.
- **IP rules** — **block or allow-only**, global or per service.
- **Rate limiting** — per-IP, global or per service.
- **Geo blocking** — by country, block or allow-only; bring GeoIP however you like (DB-IP Lite,
  MaxMind, upload, or drop-in).
- **TLS** — Let's Encrypt, internal CA, or **bring-your-own certificate**.
- **Ops** — sessions + revocable API tokens, full audit log, config snapshots + one-click rollback.

## How it works

Ward is a **control plane, not a data plane**. It configures Caddy through the admin API and reads
its logs — it is *never* in the request path, so the edge keeps serving even if Ward is down. The
**database is the single source of truth**; the Caddy config is fully regenerated from it, validated,
snapshotted, and rolled back on demand (Ward is the only writer of Caddy config). It's **API-first**,
and the web UI ships embedded in the same single Go binary.

## Quick start

Images are published to GHCR; [`deploy/`](deploy/) has a docker-compose for the whole stack.

```sh
cd deploy
cp .env.example .env         # set WARD_TAG and (optionally) WARD_ACME_EMAIL
docker compose --env-file .env up -d
```

The management plane is private by default — reach the UI over an SSH tunnel:

```sh
ssh -L 8080:localhost:8080 <your-box>   # then open http://localhost:8080
```

See [`deploy/README.md`](deploy/README.md) for the full topology, backups, and the break-glass
recovery runbook.

## Stack

Caddy + Coraza (+ CrowdSec, planned) edge, compiled into one custom `xcaddy` image · Go backend
(single binary, embedded UI) · SQLite by default, Postgres optional · React + TanStack Router UI.

## Contributing

Issues and PRs welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). Found a security issue? Please read
[SECURITY.md](SECURITY.md) first (don't open a public issue).

## License

[MIT](LICENSE) © Bhagyajit Jagdev
