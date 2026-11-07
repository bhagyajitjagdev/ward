# Changelog

All notable changes to Ward are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims for
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2026-11-07

### Added

- **Snapshots screen** — browse every applied config, view its Caddy JSON, and roll back from the UI.
- **Block expiry in the UI** — temporary IP rules (1h … 30d) from the blocklist dialog.

### Changed

- **PATCH is a partial update (JSON Merge Patch, RFC 7396)** on services, IP rules, rate limits,
  geo rules and custom WAF rules: only the fields in the body change, `null` clears a field,
  nested objects merge per field, arrays replace as a whole. Previously an omitted field was
  reset to its default with a 200 — an omitted `waf_enabled` turned the WAF off, an omitted `mode`
  flipped an allow-only IP or geo rule to block, an omitted `scope` made a per-service rule
  edge-wide. The spec's update bodies (`*Update` schemas) are now all-optional to match.
- **Rollback is durable.** A snapshot now records Ward's declarative state next to the rendered
  Caddy config; rolling back restores that state to the database and re-applies it, so the drift
  reconciler keeps it (previously a rollback only reloaded the edge and was overwritten within a
  minute). Snapshots taken before this hold no state and are reported `restorable: false` (409 on
  rollback). Identical re-applies no longer add snapshot rows.
- **Dashboards aggregate in SQL** — the Overview and access-log stats no longer load the raw event
  window into memory.
- **Contributors:** the UI compiles with **TypeScript 7**. `npm run generate:api` runs
  openapi-typescript from its own toolchain in `ui/tools/openapi/` (TypeScript 6 — TS 7 ships no
  classic compiler API), `ui/.npmrc` is gone, and CI fails if `api.schema.d.ts` drifts from
  `openapi.yaml`.

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

### Security

- **Edge stack: Coraza 3.8.1** (coraza-caddy 2.6.2, CrowdSec bouncer 0.15.0). Coraza
  3.8.1 is an upstream security release covering 3.7.0 — notably a process crash on deeply nested
  JSON bodies and a JSON memory-exhaustion path (both High), plus WAF bypasses such as `t:jsDecode`
  octal escapes and truncated multipart bodies not setting `MULTIPART_STRICT_ERROR`. Upgrade the
  edge image.
- **Caddy 2.11.9** (from 2.11.4) — upstream security hardening and fixes. Caddy 2.11.8 also
  tightened config validation (e.g. several `expression` lines in one matcher, unknown TLS
  protocol names): configs Ward renders are unaffected, but a raw-Caddyfile fragment relying on
  the old leniency is now rejected at validate, before apply. Requests with an invalid host get
  `400` before any route runs.
- **Data layer: bun 1.3.0** (plus goose 3.29, `x/crypto` 0.58) — upstream SQL-formatting
  hardening. bun now refuses text containing a NUL byte, so Ward stores request-derived event and
  audit text with any NUL shown as a literal `\x00`; otherwise a request carrying `%00` could have
  dropped itself from the WAF event log.

## [0.3.0] - 2026-08-12

### Added

- **Trusted IPs** — a global allowlist of IPs/CIDRs exempt from every edge protection (CrowdSec
  bouncer, IP blocklist, WAF, rate limits, geo blocking); managed from the Blocklist page. See the
  [release notes](https://github.com/bhagyajitjagdev/ward/releases/tag/v0.3.0).

## [0.2.0] - 2026-07-28

### Added

- **Path-based routing** (per-path backends, deny, strip-prefix), **TLS hardening** (edge-wide
  minimum version, strict SNI), **active health checks**, redirect-only services, opt-in
  **Prometheus metrics**, snapshot JSON access + declarative config export, CrowdSec live mode,
  and a redesigned service form. See the [release notes](https://github.com/bhagyajitjagdev/ward/releases/tag/v0.2.0).

## [0.1.0] - 2026-07-24

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

_Ward is pre-1.0; a minor version may include breaking changes._

[Unreleased]: https://github.com/bhagyajitjagdev/ward/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/bhagyajitjagdev/ward/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/bhagyajitjagdev/ward/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/bhagyajitjagdev/ward/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/bhagyajitjagdev/ward/releases/tag/v0.1.0
