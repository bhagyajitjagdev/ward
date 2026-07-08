# Ward control-plane image. Pure-Go (CGO off) → static binary on a minimal base.
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /ward ./cmd/ward

FROM alpine:3
RUN adduser -D -u 1000 ward
USER ward
COPY --from=build /ward /usr/local/bin/ward
ENV WARD_ADDR=":8080" \
    WARD_DB="file:/tmp/ward.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
ENTRYPOINT ["ward"]
