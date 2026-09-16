# Changelog

All notable changes to Ward are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims for
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Reverse proxy** — services (hostname → upstreams), load-balancing policies, auto-TLS.
- **WAF** (Coraza + OWASP CRS) — per-service enable, and **enforcement mode**: a global
  detection/enforce default with a per-service override.
- **Tuning assistant** — searchable WAF detections, top-triggers clustering, and one-click scoped
  exclusions applied to the live config.
- **IP rules** — block or **allow-only** (default-deny), global or per service.
- **Rate limiting** — per-IP, global or per service.
- **Geo blocking** — by country, block or allow-only; GeoIP via DB-IP Lite, MaxMind, upload, or a
  drop-in `.mmdb`.
- **TLS** — Let's Encrypt (ACME), internal CA, or **bring-your-own certificate**; configurable ACME
  contact email.
- **Ops** — authentication (sessions + revocable API tokens), a full audit log, and config snapshots
  with one-click rollback.
- **Packaging** — web UI embedded in the single Go binary; images published to GHCR; docker-compose
  deployment.
- **Snapshots screen** — browse every applied config, view its Caddy JSON, and roll back from the UI.
- **Block expiry in the UI** — temporary IP rules (1h … 30d) from the blocklist dialog.

### Changed

- **Rollback is durable.** A snapshot now records Ward's declarative state next to the rendered
  Caddy config; rolling back restores that state to the database and re-applies it, so the drift
  reconciler keeps it (previously a rollback only reloaded the edge and was overwritten within a
  minute). Snapshots taken before this hold no state and are reported `restorable: false` (409 on
  rollback). Identical re-applies no longer add snapshot rows.
- **Dashboards aggregate in SQL** — the Overview and access-log stats no longer load the raw event
  window into memory.

### Fixed

- **IPv6 over-block** — "Block IP" from a WAF event sent `<ip>/32`, which on an IPv6 client blocked a
  /32 of the v6 space; it now blocks the address itself.
- **Block edit wiped the expiry** — the blocklist edit dialog didn't round-trip `expires_at`, turning
  temporary bans permanent on any edit.
- **Cert delete guard** — an uploaded certificate still securing an enabled custom-TLS service can't be
  deleted (409); previously the host silently fell back to ACME issuance on the next reconcile.
- **Wildcard + managed TLS** is rejected at save (no DNS-01 in the edge image) instead of failing
  issuance forever.
- `ward gen-config` omitted trusted IPs; it now uses the exact render path of a live apply.

_Ward is pre-1.0; expect breaking changes until the first tagged release._

[Unreleased]: https://github.com/bhagyajitjagdev/ward/commits/main
