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
	WardJSON  string    `bun:"ward_json,notnull"` // DesiredState JSON; '' on legacy rows
	Note      string    `bun:"note"`
	Active    bool      `bun:"active,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`

	// Restorable is computed by ListSnapshots (ward_json <> '') so the list can say
	// which snapshots a rollback can restore without shipping the state itself.
	Restorable bool `bun:"restorable,scanonly"`
}

// SaveSnapshot records an applied config — the rendered Caddy JSON plus the desired
// state it came from — and marks it the active one. It is a no-op when both are
// identical to the active snapshot: the reconciler re-applies every minute, and
// without this the history would fill with one identical row per tick, pushing the
// snapshots that matter (real changes) out of the list.
func (s *Store) SaveSnapshot(ctx context.Context, cfgJSON, wardJSON []byte, note string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	return s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var active snapshotRow
		err := tx.NewSelect().Model(&active).Column("caddy_json", "ward_json").
			Where("active = ?", true).Limit(1).Scan(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && active.CaddyJSON == string(cfgJSON) && active.WardJSON == string(wardJSON) {
			return nil // nothing changed — keep the history meaningful
		}
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
			WardJSON:  string(wardJSON),
			Note:      note,
			Active:    true,
			CreatedAt: time.Now().UTC(),
		}
		_, err = tx.NewInsert().Model(row).Exec(ctx)
		return err
	})
}

func (r snapshotRow) toModel() model.ConfigSnapshot {
	return model.ConfigSnapshot{
		ID: r.ID, Note: r.Note, Active: r.Active, CreatedAt: r.CreatedAt,
		Restorable: r.Restorable || r.WardJSON != "",
		CaddyJSON:  r.CaddyJSON, WardJSON: r.WardJSON,
	}
}

// ListSnapshots returns snapshot metadata (no config bodies), newest first.
func (s *Store) ListSnapshots(ctx context.Context) ([]model.ConfigSnapshot, error) {
	var rows []snapshotRow
	err := s.DB.NewSelect().Model(&rows).
		Column("id", "note", "active", "created_at").
		ColumnExpr("(cs.ward_json <> '') AS restorable").
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

// GetSnapshot returns a full snapshot (including CaddyJSON + WardJSON), or nil if
// not found.
func (s *Store) GetSnapshot(ctx context.Context, id string) (*model.ConfigSnapshot, error) {
	var row snapshotRow
	err := s.DB.NewSelect().Model(&row).
		Column("id", "caddy_json", "ward_json", "note", "active", "created_at").
		Where("id = ?", id).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := row.toModel()
	return &m, nil
}
