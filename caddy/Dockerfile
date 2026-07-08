# Custom Caddy with the OWASP Coraza WAF compiled in (spike).
# Same xcaddy pattern Ward's real caddy/ image will use — here, Coraza only
# (no CrowdSec bouncer / DNS modules yet).
#
# If the 2.11.4 tags are unavailable at build time, bump to the current stable
# Caddy release (check https://github.com/caddyserver/caddy/releases).

FROM caddy:2.11.4-builder AS builder

RUN xcaddy build \
    --with github.com/corazawaf/coraza-caddy/v2

FROM caddy:2.11.4

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
