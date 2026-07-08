-- +goose Up
-- mode: 'block' = deny these entries (default); 'allow' = deny everything NOT listed.
ALTER TABLE blocklist ADD COLUMN mode TEXT NOT NULL DEFAULT 'block';
ALTER TABLE geo_rules ADD COLUMN mode TEXT NOT NULL DEFAULT 'block';

-- +goose Down
ALTER TABLE geo_rules DROP COLUMN mode;
ALTER TABLE blocklist DROP COLUMN mode;
