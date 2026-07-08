package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type blockRow struct {
	bun.BaseModel `bun:"table:blocklist,alias:bl"`

	ID        string     `bun:"id,pk"`
	Scope     string     `bun:"scope,notnull"`
	Mode      string     `bun:"mode,notnull"`
	ServiceID *string    `bun:"service_id"`
	CIDR      string     `bun:"cidr,notnull"`
	Reason    string     `bun:"reason"`
	Source    string     `bun:"source,notnull"`
	ExpiresAt *time.Time `bun:"expires_at"`
	CreatedAt time.Time  `bun:"created_at,notnull"`
}

func (r blockRow) toModel() model.BlockedIP {
	return model.BlockedIP{
		ID: r.ID, Scope: r.Scope, Mode: r.Mode, ServiceID: r.ServiceID, CIDR: r.CIDR,
		Reason: r.Reason, Source: r.Source, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
	}
}

// CreateBlock inserts a block (server-assigned id + timestamp).
func (s *Store) CreateBlock(ctx context.Context, in model.BlockedIP) (model.BlockedIP, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.BlockedIP{}, err
	}
	row := blockRow{
		ID: id.String(), Scope: orDefault(in.Scope, "global"), Mode: orDefault(in.Mode, "block"),
		ServiceID: in.ServiceID, CIDR: in.CIDR, Reason: in.Reason, Source: orDefault(in.Source, "manual"),
		ExpiresAt: in.ExpiresAt, CreatedAt: time.Now().UTC(),
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.BlockedIP{}, err
	}
	return row.toModel(), nil
}

// ListBlocks returns all blocks, newest first.
func (s *Store) ListBlocks(ctx context.Context) ([]model.BlockedIP, error) {
	return s.listBlocks(ctx, false)
}

// ListActiveBlocks returns non-expired blocks (for config generation).
func (s *Store) ListActiveBlocks(ctx context.Context) ([]model.BlockedIP, error) {
	return s.listBlocks(ctx, true)
}

func (s *Store) listBlocks(ctx context.Context, activeOnly bool) ([]model.BlockedIP, error) {
	var rows []blockRow
	q := s.DB.NewSelect().Model(&rows).Order("created_at DESC")
	if activeOnly {
		q = q.Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC())
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.BlockedIP, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// DeleteBlock removes a block; reports whether one was found.
func (s *Store) DeleteBlock(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*blockRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
