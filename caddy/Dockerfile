# Custom Caddy with the OWASP Coraza WAF + the CrowdSec bouncer compiled in.
# Caddy has no runtime plugins, so every edge capability is baked in here via xcaddy.
#
# If the 2.11.4 tags are unavailable at build time, bump to the current stable
# Caddy release (check https://github.com/caddyserver/caddy/releases).

FROM caddy:2.11.4-builder AS builder

RUN xcaddy build \
    --with github.com/corazawaf/coraza-caddy/v2 \
    --with github.com/mholt/caddy-ratelimit \
    --with github.com/porech/caddy-maxmind-geolocation \
    --with github.com/hslatman/caddy-crowdsec-bouncer/http \
    --with github.com/hslatman/caddy-crowdsec-bouncer/crowdsec

FROM caddy:2.11.4

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
