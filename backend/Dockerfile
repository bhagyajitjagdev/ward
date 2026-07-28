# ward — control plane (API + embedded ward-ui).
# Build with the repo root as context:  docker build -f backend/Dockerfile .

# 1. Build the UI.
FROM node:22-alpine AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ui/.npmrc ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# 2. Build the Go binary with the UI embedded (CGO off → static).
FROM golang:1.25 AS build
ARG VERSION=dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=ui /ui/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -tags embed_ui -ldflags="-s -w -X main.version=${VERSION}" -o /ward ./cmd/ward

# 2b. Build the plugin-enabled Caddy — SAME pins as caddy/Dockerfile (kept in lockstep
#     by internal/edge/versions_test.go). Ward uses it to `caddy adapt` a service's
#     advanced Caddyfile fragment, so plugin directives (rate_limit, coraza_waf, …) work
#     in fragments, matching the edge — not just core directives.
FROM caddy:2.11.4-builder AS caddybuild
RUN xcaddy build \
    --with github.com/corazawaf/coraza-caddy/v2@v2.5.0 \
    --with github.com/corazawaf/coraza/v3@v3.7.0 \
    --with github.com/corazawaf/coraza-coreruleset/v4@v4.25.0 \
    --with github.com/mholt/caddy-ratelimit@v0.1.0 \
    --with github.com/porech/caddy-maxmind-geolocation@v1.0.3 \
    --with github.com/hslatman/caddy-crowdsec-bouncer/http@v0.13.1 \
    --with github.com/hslatman/caddy-crowdsec-bouncer/crowdsec@v0.13.1

# 3. Runtime. Runs as root so it can write the shared volumes (db/geoip/certs);
#    non-root hardening (entrypoint chown + gosu) is a documented follow-up.
FROM alpine:3
RUN apk add --no-cache ca-certificates wget && mkdir -p /data /geoip /certs /waf
COPY --from=build /ward /usr/local/bin/ward
# The plugin-enabled Caddy (built above) — used to `caddy adapt` a service's advanced
# Caddyfile fragment into JSON, so fragments can use the same directives as the edge.
COPY --from=caddybuild /usr/bin/caddy /usr/local/bin/caddy
ENV WARD_ADDR=":8080" \
    WARD_UI="1" \
    WARD_DB="file:/data/ward.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
EXPOSE 8080
ENTRYPOINT ["ward"]
