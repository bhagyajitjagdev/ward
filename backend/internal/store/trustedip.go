package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type trustedRow struct {
	bun.BaseModel `bun:"table:trusted_ips,alias:ti"`

	ID        string    `bun:"id,pk"`
	CIDR      string    `bun:"cidr,notnull"`
	Note      string    `bun:"note"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

func (r trustedRow) toModel() model.TrustedIP {
	return model.TrustedIP{ID: r.ID, CIDR: r.CIDR, Note: r.Note, CreatedAt: r.CreatedAt}
}

// CreateTrusted inserts a trusted IP/CIDR (server-assigned id + timestamp).
func (s *Store) CreateTrusted(ctx context.Context, in model.TrustedIP) (model.TrustedIP, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.TrustedIP{}, err
	}
	row := trustedRow{ID: id.String(), CIDR: in.CIDR, Note: in.Note, CreatedAt: time.Now().UTC()}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.TrustedIP{}, err
	}
	return row.toModel(), nil
}

// ListTrusted returns all trusted IPs, newest first.
func (s *Store) ListTrusted(ctx context.Context) ([]model.TrustedIP, error) {
	var rows []trustedRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.TrustedIP, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// DeleteTrusted removes a trusted entry; reports whether one was found.
func (s *Store) DeleteTrusted(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*trustedRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
