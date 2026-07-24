-- +goose Up
-- Per-service HTTP/proxy controls. http_config is a JSON blob of the structured
-- knobs (headers + security preset, basic auth, path rewrite, compression…);
-- raw_caddy is the advanced escape hatch (a Caddyfile fragment adapted + spliced
-- into the service route). Both rendered into the generated config; DB is truth.
ALTER TABLE services ADD COLUMN http_config TEXT NOT NULL DEFAULT '{}';
ALTER TABLE services ADD COLUMN raw_caddy TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE services DROP COLUMN http_config;
ALTER TABLE services DROP COLUMN raw_caddy;
