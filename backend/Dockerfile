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

# 3. Runtime. Runs as root so it can write the shared volumes (db/geoip/certs);
#    non-root hardening (entrypoint chown + gosu) is a documented follow-up.
FROM alpine:3
RUN apk add --no-cache ca-certificates wget && mkdir -p /data /geoip /certs /waf
COPY --from=build /ward /usr/local/bin/ward
ENV WARD_ADDR=":8080" \
    WARD_UI="1" \
    WARD_DB="file:/data/ward.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
EXPOSE 8080
ENTRYPOINT ["ward"]
