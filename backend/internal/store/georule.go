package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type geoRuleRow struct {
	bun.BaseModel `bun:"table:geo_rules,alias:gr"`

	ID        string    `bun:"id,pk"`
	Scope     string    `bun:"scope,notnull"`
	ServiceID *string   `bun:"service_id"`
	Countries string    `bun:"countries,notnull"` // JSON array
	CreatedAt time.Time `bun:"created_at,notnull"`
}

func (r geoRuleRow) toModel() model.GeoRule {
	cs := []string{}
	if r.Countries != "" {
		_ = json.Unmarshal([]byte(r.Countries), &cs)
	}
	return model.GeoRule{
		ID: r.ID, Scope: r.Scope, ServiceID: r.ServiceID, Countries: cs, CreatedAt: r.CreatedAt,
	}
}

// CreateGeoRule inserts a geo rule (server-assigned id + timestamp).
func (s *Store) CreateGeoRule(ctx context.Context, in model.GeoRule) (model.GeoRule, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.GeoRule{}, err
	}
	cs, err := json.Marshal(orEmpty(in.Countries))
	if err != nil {
		return model.GeoRule{}, err
	}
	row := geoRuleRow{
		ID: id.String(), Scope: orDefault(in.Scope, "global"), ServiceID: in.ServiceID,
		Countries: string(cs), CreatedAt: time.Now().UTC(),
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.GeoRule{}, err
	}
	return row.toModel(), nil
}

// ListGeoRules returns all geo rules, newest first.
func (s *Store) ListGeoRules(ctx context.Context) ([]model.GeoRule, error) {
	var rows []geoRuleRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.GeoRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// DeleteGeoRule removes a geo rule; reports whether one was found.
func (s *Store) DeleteGeoRule(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*geoRuleRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
