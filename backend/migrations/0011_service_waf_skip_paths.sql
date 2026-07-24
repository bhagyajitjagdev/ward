-- +goose Up
-- Per-service WAF skip-paths: request paths for which the Coraza handler is bypassed
-- entirely, so streaming endpoints (SSE) work — the handler buffers responses and
-- blocks connection upgrades whenever it's in the request path, which no SecLang
-- directive can undo. Stored as a JSON array; each entry matches the path and its
-- subpaths. WebSocket upgrades are auto-bypassed by config-gen regardless of this
-- list. Other protections (IP blocklist, geo, rate-limit) still apply to these paths.
ALTER TABLE services ADD COLUMN waf_skip_paths TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE services DROP COLUMN waf_skip_paths;
