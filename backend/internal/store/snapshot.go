package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// ErrNotFound is returned when a looked-up row does not exist.
var ErrNotFound = errors.New("not found")

type snapshotRow struct {
	bun.BaseModel `bun:"table:config_snapshots,alias:cs"`

	ID        string    `bun:"id,pk"`
	CaddyJSON string    `bun:"caddy_json,notnull"`
	Note      string    `bun:"note"`
	Active    bool      `bun:"active,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

// SaveSnapshot records an applied Caddy config and marks it the active one
// (deactivating any previous active snapshot). This is the rollback history.
func (s *Store) SaveSnapshot(ctx context.Context, cfgJSON []byte) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().
			Table("config_snapshots").
			Set("active = ?", false).
			Where("active = ?", true).
			Exec(ctx); err != nil {
			return err
		}
		row := &snapshotRow{
			ID:        id.String(),
			CaddyJSON: string(cfgJSON),
			Active:    true,
			CreatedAt: time.Now().UTC(),
		}
		_, err := tx.NewInsert().Model(row).Exec(ctx)
		return err
	})
}

func (r snapshotRow) toModel() model.ConfigSnapshot {
	return model.ConfigSnapshot{ID: r.ID, Note: r.Note, Active: r.Active, CreatedAt: r.CreatedAt, CaddyJSON: r.CaddyJSON}
}

// ListSnapshots returns snapshot metadata (no config body), newest first.
func (s *Store) ListSnapshots(ctx context.Context) ([]model.ConfigSnapshot, error) {
	var rows []snapshotRow
	err := s.DB.NewSelect().Model(&rows).
		Column("id", "note", "active", "created_at").
		Order("created_at DESC").Limit(100).Scan(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.ConfigSnapshot, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// GetSnapshot returns a full snapshot (including CaddyJSON), or nil if not found.
func (s *Store) GetSnapshot(ctx context.Context, id string) (*model.ConfigSnapshot, error) {
	var row snapshotRow
	err := s.DB.NewSelect().Model(&row).Where("id = ?", id).Limit(1).Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := row.toModel()
	return &m, nil
}

// SetActiveSnapshot marks one snapshot active (deactivating the rest).
func (s *Store) SetActiveSnapshot(ctx context.Context, id string) error {
	return s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Table("config_snapshots").
			Set("active = ?", false).Where("active = ?", true).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewUpdate().Table("config_snapshots").
			Set("active = ?", true).Where("id = ?", id).Exec(ctx)
		return err
	})
}
