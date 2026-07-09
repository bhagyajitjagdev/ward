-- +goose Up
-- Raw HTTP access events tailed from Caddy's structured access log. Short-retention
-- (a few days) — the long-term / searchable store is an external log pipeline
-- (Promtail/Vector -> Loki -> Grafana), fed by the same log file. Ward keeps just
-- enough to power the dashboard + an in-UI recent-requests view.
CREATE TABLE access_events (
    id          TEXT PRIMARY KEY,
    ts          TIMESTAMP NOT NULL,
    service_id  TEXT REFERENCES services(id) ON DELETE SET NULL,
    host        TEXT NOT NULL,
    client_ip   TEXT NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    query       TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL,
    duration_ms REAL NOT NULL,
    bytes       BIGINT NOT NULL DEFAULT 0,
    user_agent  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_access_events_ts ON access_events (ts);
CREATE INDEX idx_access_events_service_ts ON access_events (service_id, ts);

-- +goose Down
DROP TABLE access_events;
