-- +goose Up
-- Trusted IPs / CIDRs: exempt from every edge threat protection (CrowdSec, IP
-- blocklist, WAF, rate-limits, geo). Global — a trusted address is trusted for the
-- whole edge. Dialect-portable.
CREATE TABLE trusted_ips (
    id         TEXT PRIMARY KEY,
    cidr       TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE trusted_ips;
