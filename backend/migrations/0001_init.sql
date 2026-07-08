-- +goose Up
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin',
    is_owner      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP NOT NULL
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE services (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    public_hostname TEXT NOT NULL UNIQUE,
    upstreams       TEXT NOT NULL,
    lb_policy       TEXT NOT NULL DEFAULT 'round_robin',
    tls_mode        TEXT NOT NULL DEFAULT 'internal',
    waf_enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

CREATE TABLE config_snapshots (
    id         TEXT PRIMARY KEY,
    caddy_json TEXT NOT NULL,
    note       TEXT,
    applied_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    active     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE audit_log (
    id         TEXT PRIMARY KEY,
    actor      TEXT,
    action     TEXT NOT NULL,
    target     TEXT,
    detail     TEXT,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE waf_events (
    id               TEXT PRIMARY KEY,
    tx_id            TEXT NOT NULL,
    ts               TIMESTAMP NOT NULL,
    service_id       TEXT REFERENCES services(id) ON DELETE SET NULL,
    host             TEXT,
    client_ip        TEXT,
    authed           BOOLEAN NOT NULL DEFAULT FALSE,
    user_agent       TEXT,
    method           TEXT,
    path             TEXT NOT NULL,
    uri              TEXT,
    status           INTEGER,
    engine_mode      TEXT,
    is_interrupted   BOOLEAN NOT NULL DEFAULT FALSE,
    rule_id          INTEGER,
    rule_msg         TEXT,
    severity         TEXT,
    matched_target   TEXT,
    matched_value    TEXT,
    tags             TEXT,
    is_anomaly_score BOOLEAN NOT NULL DEFAULT FALSE,
    crs_version      TEXT,
    raw              TEXT
);
CREATE INDEX idx_waf_events_cluster ON waf_events (service_id, path, rule_id);
CREATE INDEX idx_waf_events_ts ON waf_events (ts);

CREATE TABLE waf_exclusions (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL DEFAULT 'service',
    service_id TEXT REFERENCES services(id) ON DELETE SET NULL,
    rule_id    INTEGER,
    path       TEXT,
    target     TEXT,
    seclang    TEXT NOT NULL,
    state      TEXT NOT NULL DEFAULT 'draft',
    soak_until TIMESTAMP,
    source     TEXT NOT NULL DEFAULT 'manual',
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE blocklist (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL DEFAULT 'global',
    service_id TEXT REFERENCES services(id) ON DELETE SET NULL,
    cidr       TEXT NOT NULL,
    reason     TEXT,
    source     TEXT NOT NULL DEFAULT 'manual',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE api_tokens (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    user_id      TEXT REFERENCES users(id) ON DELETE CASCADE,
    last_used_at TIMESTAMP,
    expires_at   TIMESTAMP,
    revoked      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMP NOT NULL
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE settings;
DROP TABLE api_tokens;
DROP TABLE blocklist;
DROP TABLE waf_exclusions;
DROP TABLE waf_events;
DROP TABLE audit_log;
DROP TABLE config_snapshots;
DROP TABLE services;
DROP TABLE sessions;
DROP TABLE users;
