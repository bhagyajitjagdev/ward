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

_Ward is pre-1.0; expect breaking changes until the first tagged release._

[Unreleased]: https://github.com/bhagyajitjagdev/ward/commits/main
