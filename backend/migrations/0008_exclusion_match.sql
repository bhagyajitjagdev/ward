-- +goose Up
-- Richer scoped exclusions: a path match-type (exact/prefix/regex) and an optional
-- HTTP-method filter, so the generated SecLang can be a chained condition rather
-- than only a path prefix. Existing rows default to the prior behavior (prefix, no
-- method filter). methods is a comma-joined uppercase list ('' = any method).
ALTER TABLE waf_exclusions ADD COLUMN path_match TEXT NOT NULL DEFAULT 'prefix';
ALTER TABLE waf_exclusions ADD COLUMN methods TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE waf_exclusions DROP COLUMN path_match;
ALTER TABLE waf_exclusions DROP COLUMN methods;
