package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type rateLimitRow struct {
	bun.BaseModel `bun:"table:rate_limits,alias:rl"`

	ID        string    `bun:"id,pk"`
	Scope     string    `bun:"scope,notnull"`
	ServiceID *string   `bun:"service_id"`
	MaxEvents int       `bun:"max_events,notnull"`
	Window    string    `bun:"window_dur,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

func (r rateLimitRow) toModel() model.RateLimit {
	return model.RateLimit{
		ID: r.ID, Scope: r.Scope, ServiceID: r.ServiceID,
		MaxEvents: r.MaxEvents, Window: r.Window, CreatedAt: r.CreatedAt,
	}
}

// CreateRateLimit inserts a rate limit (server-assigned id + timestamp).
func (s *Store) CreateRateLimit(ctx context.Context, in model.RateLimit) (model.RateLimit, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.RateLimit{}, err
	}
	row := rateLimitRow{
		ID: id.String(), Scope: orDefault(in.Scope, "global"), ServiceID: in.ServiceID,
		MaxEvents: in.MaxEvents, Window: in.Window, CreatedAt: time.Now().UTC(),
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.RateLimit{}, err
	}
	return row.toModel(), nil
}

// ListRateLimits returns all rate limits, newest first.
func (s *Store) ListRateLimits(ctx context.Context) ([]model.RateLimit, error) {
	var rows []rateLimitRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.RateLimit, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// DeleteRateLimit removes a rate limit; reports whether one was found.
func (s *Store) DeleteRateLimit(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*rateLimitRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
