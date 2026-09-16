-- +goose Up
-- The declarative desired state (store.DesiredState JSON) each config was generated
-- from. Lets a rollback restore the DB — the source of truth — not just the live edge,
-- which the drift reconciler would otherwise overwrite within a minute. Pre-existing
-- snapshots stay '' (not restorable).
ALTER TABLE config_snapshots ADD COLUMN ward_json TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE config_snapshots DROP COLUMN ward_json;
