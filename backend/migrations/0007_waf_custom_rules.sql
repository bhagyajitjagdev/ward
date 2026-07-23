-- +goose Up
-- User-authored raw SecLang (advanced escape hatch beyond the generated
-- exclusions). Rendered into the WAF directives in the before-CRS slot, after
-- Ward's generated exclusions; validated by pushing the regenerated config to
-- Caddy before the row is kept (Caddy load is atomic — a bad rule never sticks).
CREATE TABLE waf_custom_rules (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL DEFAULT 'global',           -- 'global' | 'service'
    service_id TEXT REFERENCES services(id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    seclang    TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE waf_custom_rules;
