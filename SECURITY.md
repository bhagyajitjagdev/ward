# Security Policy

Ward is a security tool, so responsible disclosure is genuinely appreciated.

## Reporting a vulnerability

**Please don't open a public issue for security problems.** Use GitHub's private vulnerability
reporting instead:

> Repo **Security** tab → **Report a vulnerability**
> (enable it under **Settings → Code security and analysis → Private vulnerability reporting** if it
> isn't already on.)

I'll acknowledge your report as quickly as I can and keep you posted on a fix and disclosure timeline.

## Deployment security notes

These are load-bearing invariants — treat a regression in any of them as a security bug:

- The **Caddy admin API (`:2019`) is unauthenticated by design** and must be kept on an internal
  network — never expose it.
- The **Ward management API/UI is private** — its own port, reached over an SSH tunnel or a
  mesh/VPN, never behind the public edge.
- The **database is the whole edge's source of truth** — protect it and back it up (a copy of the DB
  volume is a full backup).

## Supported versions

Ward is pre-1.0. Only the latest `main` / newest release receives security fixes.
