# Custom Caddy with the OWASP Coraza WAF + the CrowdSec bouncer compiled in.
# Caddy has no runtime plugins, so every edge capability is baked in here via xcaddy.
#
# Module versions are PINNED for reproducible builds — a bump is an explicit, reviewable
# diff (Renovate opens the PR; CI builds + runs the e2e suite; a human merges). Caddy
# itself is pinned by the base-image tag; bump it alongside a release when needed
# (https://github.com/caddyserver/caddy/releases). NOTE: the CrowdSec *agent* and
# Postgres are separate, user-chosen pulled containers — not part of this image.

FROM caddy:2.11.4-builder AS builder

# Pin every module explicitly (coraza/v3 + the CRS are pinned too, not left transitive)
# so the compiled versions are exactly what the labels below claim.
RUN xcaddy build \
    --with github.com/corazawaf/coraza-caddy/v2@v2.5.0 \
    --with github.com/corazawaf/coraza/v3@v3.7.0 \
    --with github.com/corazawaf/coraza-coreruleset/v4@v4.25.0 \
    --with github.com/mholt/caddy-ratelimit@v0.1.0 \
    --with github.com/porech/caddy-maxmind-geolocation@v1.0.3 \
    --with github.com/hslatman/caddy-crowdsec-bouncer/http@v0.13.1 \
    --with github.com/hslatman/caddy-crowdsec-bouncer/crowdsec@v0.13.1

FROM caddy:2.11.4

COPY --from=builder /usr/bin/caddy /usr/bin/caddy

# The product version — a Ward release tags both images the same; CI passes it in.
ARG WARD_VERSION=dev

# The exact edge stack baked into this image — inspect with:
#   docker inspect ghcr.io/bhagyajitjagdev/ward-caddy:<tag> --format '{{json .Config.Labels}}'
LABEL org.opencontainers.image.title="ward-caddy" \
      org.opencontainers.image.description="Ward edge — Caddy + Coraza WAF + CrowdSec bouncer" \
      org.opencontainers.image.source="https://github.com/bhagyajitjagdev/ward" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${WARD_VERSION}" \
      io.ward.caddy="v2.11.4" \
      io.ward.coraza="v3.7.0" \
      io.ward.coraza_caddy="v2.5.0" \
      io.ward.crs="v4.25.0" \
      io.ward.crowdsec_bouncer="v0.13.1" \
      io.ward.ratelimit="v0.1.0" \
      io.ward.maxmind_geolocation="v1.0.3"
