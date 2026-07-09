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

## Security invariants

- **Caddy admin API (`:2019`) is never published** — it's unauthenticated; keep it on the
  internal compose network only (this compose does).
- **The management plane (`ward:8080`) is private** — bound to `127.0.0.1` here; reach it via SSH
  tunnel or a mesh/VPN interface. Never put it behind the public Caddy.
- **Backup = copy the `ward_db` volume.** It's the whole edge's source of truth. Custom certs live
  on the `certs` volume; Caddy's ACME/internal certs on `caddy_data`.

## Break-glass recovery

A bad rule (e.g. a global allow-list that excludes your own IP) blocks traffic **through Caddy**,
but never the Ward API — it's on its own port, not behind Caddy. SSH to Box A and talk to Ward
directly:

```sh
# get a token
TOKEN=$(curl -s localhost:8080/api/auth/login -H 'content-type: application/json' \
  -d '{"username":"owner","password":"…"}' | jq -r .token)
H="authorization: Bearer $TOKEN"

# option 1 — roll the live config back to the last-good snapshot
curl -s "$H" localhost:8080/api/config-snapshots            # find an id
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
