# Deploying Ward

Runs the full stack on one host (**Box A**): `ward-caddy` (the edge) + `ward` (control
plane + embedded UI). Images are pulled from GHCR — build/push happens in CI on every push
to `main` and every `v*` tag.

## Bring-up

```sh
cp .env.example .env      # set WARD_TAG (latest / vX.Y.Z) and WARD_ACME_EMAIL
docker compose --env-file .env pull
docker compose --env-file .env up -d
```

First run: Ward migrates its DB and reconciles an (empty) config to Caddy. Open the UI via an
SSH tunnel (the management plane is **not** public):

```sh
ssh -L 8080:localhost:8080 <box-a>     # then browse http://localhost:8080
```

Create the owner account, add services (their upstreams point at **Box B**, below).

## Updating

```sh
# edit WARD_TAG in .env (or keep :latest), then:
docker compose --env-file .env pull && docker compose --env-file .env up -d
```

## Database: SQLite or Postgres

Ward picks its database from **`WARD_DB`** — no separate flag. Unset (the default) → a SQLite file on
the `ward_db` volume. A `postgres://…` DSN → Postgres (bun's `pgdialect`; the same dual-dialect goose
migrations run on both — verified against Postgres 16).

To use Postgres: in `.env`, set `WARD_DB` + `POSTGRES_PASSWORD`, then bring it up with the profile:

```sh
docker compose --env-file .env --profile postgres up -d
```

Ward retries the DB on startup, so it's fine that Postgres comes up second. Backup unit: the `pgdata`
volume (or `ward_db` in SQLite mode).

## The 2-box topology

- **Box A** — this stack. Public `:80`/`:443`; management plane private.
- **Box B** — lightweight apps (whoami, httpbin, a demo API, a deliberately-vulnerable app for
  WAF demos). Each service you create in Ward has upstreams pointing at Box B
  (`box-b.mesh:port`, or a private IP:port). Only Box A's `:80`/`:443` face the internet; public
  DNS for your test hostnames → Box A's public IP (needed for real Let's Encrypt).

## Streaming: WebSocket & SSE

A WAF sits in the request path and inspects responses, which can break streaming. Ward handles
the two common cases so nothing needs configuring:

- **WebSocket — automatic bypass.** Any request with `Upgrade: websocket` skips the WAF. A WAF
  can't inspect WebSocket frames anyway (only the handshake), so this gives up nothing.
- **Server-Sent Events — streams through the WAF.** Since the edge moved to Coraza-Caddy 2.6.1 the
  handler flushes as the upstream writes, so SSE endpoints stream normally *with* request
  inspection and enforcement intact (an e2e check guards this).

**Fallback: skip paths.** If some other streaming or long-poll endpoint still buffers, add it under
**WAF → Skip paths** in the service form; the WAF is bypassed for that path **and its subpaths**.
The IP blocklist, geo and rate-limit still apply there — only Coraza is skipped. Scope it to the
exact endpoints (not `/api/*`): a skipped path loses WAF request inspection.

## Environment variables

Set on the `ward` service (see [`docker-compose.yml`](docker-compose.yml)); most have sensible
defaults, and the deploy-facing ones are in [`.env.example`](.env.example).

| Variable | Default | Purpose |
|---|---|---|
| `WARD_DB` | SQLite on `ward_db` | Database. A `postgres://…` DSN switches to Postgres. |
| `WARD_ADDR` | `:8080` | Management API + UI listen address — keep **private**. |
| `WARD_ACME_EMAIL` | — | Contact email for Let's Encrypt (also settable in the UI). |
| `WARD_HTTP_PORT` | `:80` | Edge HTTP listen port — override for dev / rootless hosts that can't bind `:80`. |
| `WARD_HTTPS_PORT` | `:443` | Edge HTTPS listen port. |
| `WARD_CADDY_AUTO_HTTPS` | off | Set `1` to enable Caddy automatic HTTPS (ACME issuance + HTTP→HTTPS redirects). |
| `WARD_CADDY_ADMIN` | `http://localhost:2019` | Caddy admin API URL Ward drives — **unauthenticated; never publish `:2019`.** |
| `WARD_WAF_ENGINE` | `DetectionOnly` | Initial global WAF mode (`DetectionOnly` / `On`); also managed in the UI (Settings). |
| `WARD_CROWDSEC_API_URL` / `WARD_CROWDSEC_API_KEY` | — | CrowdSec LAPI URL + bouncer key (both present ⇒ CrowdSec enabled). |
| `WARD_ACCESS_LOG` / `WARD_WAF_AUDIT_LOG` | volume paths | Where Caddy writes the access / Coraza-audit JSON logs. |

## Backup & restore

The DB is the source of truth for **config, auth, secrets, and history** — but **not TLS certs**, which
live on separate volumes. A complete backup is **two volumes**; restore drops them back and restarts, and
Ward re-derives the Caddy config from the DB on boot.

| Volume | Contains | Back up? |
|---|---|---|
| **`ward_db`** (or **`pgdata`** in Postgres mode) | All services / WAF rules / blocks / rate-limits / geo / settings, **users + API tokens**, basic-auth **hashes**, and history (audit, WAF/access events, snapshots) | **Yes — the essential one** |
| **`certs`** | Custom (bring-your-own) TLS certificates | **Yes, if you upload custom certs** |
| `caddy_data` | Let's Encrypt + internal-CA certs | Optional — Caddy re-issues via ACME on boot and auto-renews. Back up only to skip re-issuance (and Let's Encrypt rate limits with many domains). |
| `geoip` · `waf_audit` · `crowdsec_*` | GeoIP DB, access/audit logs, CrowdSec state | No — re-downloaded / regenerated |

```sh
# Volumes are prefixed by the compose project — find yours with `docker volume ls`.
DB=<project>_ward_db  CERTS=<project>_certs

# Ward isn't in the request path, so stopping it for a clean copy never drops traffic (Caddy keeps serving).
docker compose --env-file .env stop ward
docker run --rm -v $DB:/v    -v "$PWD:/out" alpine tar czf /out/ward-db.tgz    -C /v .
docker run --rm -v $CERTS:/v -v "$PWD:/out" alpine tar czf /out/ward-certs.tgz -C /v .
docker compose --env-file .env start ward

# Restore: recreate the volumes, extract, bring the stack up.
docker run --rm -v $DB:/v    -v "$PWD:/in" alpine sh -c 'cd /v && tar xzf /in/ward-db.tgz'
docker run --rm -v $CERTS:/v -v "$PWD:/in" alpine sh -c 'cd /v && tar xzf /in/ward-certs.tgz'
docker compose --env-file .env up -d
```

> The config **export** in Settings (Settings → Configuration) is a **git/diff artifact, not a backup** — it
> omits secrets, certs, auth, and history. For a real backup, copy the volumes above.

## Security invariants

- **Caddy admin API (`:2019`) is never published** — it's unauthenticated; keep it on the
  internal compose network only (this compose does).
- **The management plane (`ward:8080`) is private** — bound to `127.0.0.1` here; reach it via SSH
  tunnel or a mesh/VPN interface. Never put it behind the public Caddy.
- **Back up `ward_db` + `certs`** — see **Backup & restore** above.

## Break-glass recovery

A bad rule (e.g. a global allow-list that excludes your own IP) blocks traffic **through Caddy**,
but never the Ward API — it's on its own port, not behind Caddy. SSH to Box A and talk to Ward
directly:

```sh
# get a token
TOKEN=$(curl -s localhost:8080/api/auth/login -H 'content-type: application/json' \
  -d '{"username":"owner","password":"…"}' | jq -r .token)
H="authorization: Bearer $TOKEN"

# option 1 — roll back to the last-good snapshot (also the Snapshots screen in the UI).
# This restores Ward's own config to that point and re-applies it, so it sticks —
# every change since (the bad rule included) is undone, not just hidden from the edge.
curl -s "$H" localhost:8080/api/config-snapshots            # find an id (restorable: true)
curl -s "$H" -X POST localhost:8080/api/config-snapshots/<id>/rollback

# option 2 — delete the offending rule, Ward reapplies
curl -s "$H" -X DELETE localhost:8080/api/blocklist/<id>
```

Last resorts, in order: edit the DB row directly in the `ward_db` volume and restart; or load a
config into Caddy's admin API (`localhost:2019`) by hand. It's very hard to brick.

## Access logs → Loki / Grafana

Ward tails Caddy's structured JSON access log into its own DB, but keeps only a few days
(Settings → Access-log retention) — enough for the in-UI **Access Log** screen. For long-term,
searchable logs and richer dashboards, ship the same log file to Loki:

1. The stack already writes the access log to `/waf/access.json` on the shared `waf_audit` volume.
2. Run **Promtail** (or Alloy/Vector) alongside the stack, tailing that file → Loki. A starter config
   is in [`promtail.example.yml`](promtail.example.yml) — mount `waf_audit:/waf:ro`, point it at your
   Loki, and go.
3. Query in Grafana, e.g.:
   ```logql
   {job="ward-access"} | json                          # all requests
   {job="ward-access", status="500"}                   # server errors
   sum(rate({job="ward-access"}[5m])) by (host)         # req/s per service
   ```

Ward and Loki read the *same* file — they don't conflict. Ward gives the at-a-glance view; Grafana
gives the deep, long-term one.

## Notes

- `ward` currently runs as **root** in the container so it can write the shared volumes without a
  chown dance. Non-root hardening (entrypoint `chown` + `gosu`) is a planned follow-up.
- CrowdSec (agent + bouncer) and Postgres are not in this compose yet — SQLite is the default DB
  and CrowdSec is a separate backlog item.
