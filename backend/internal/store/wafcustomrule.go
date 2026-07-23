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

type wafCustomRuleRow struct {
	bun.BaseModel `bun:"table:waf_custom_rules,alias:wcr"`

	ID        string    `bun:"id,pk"`
	Scope     string    `bun:"scope,notnull"`
	ServiceID *string   `bun:"service_id"`
	Name      string    `bun:"name,notnull"`
	SecLang   string    `bun:"seclang,notnull"`
	Enabled   bool      `bun:"enabled,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

func (r wafCustomRuleRow) toModel() model.WAFCustomRule {
	return model.WAFCustomRule{
		ID: r.ID, Scope: r.Scope, ServiceID: r.ServiceID, Name: r.Name,
		SecLang: r.SecLang, Enabled: r.Enabled, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// CreateWAFCustomRule inserts a rule (server-assigned id + timestamps).
func (s *Store) CreateWAFCustomRule(ctx context.Context, in model.WAFCustomRule) (model.WAFCustomRule, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.WAFCustomRule{}, err
	}
	now := time.Now().UTC()
	row := wafCustomRuleRow{
		ID: id.String(), Scope: orDefault(in.Scope, "global"), ServiceID: in.ServiceID,
		Name: in.Name, SecLang: in.SecLang, Enabled: in.Enabled,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.WAFCustomRule{}, err
	}
	return row.toModel(), nil
}

// ListWAFCustomRules returns all custom rules, newest first.
func (s *Store) ListWAFCustomRules(ctx context.Context) ([]model.WAFCustomRule, error) {
	var rows []wafCustomRuleRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.WAFCustomRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// GetWAFCustomRule returns one rule; ok=false when it doesn't exist.
func (s *Store) GetWAFCustomRule(ctx context.Context, id string) (model.WAFCustomRule, bool, error) {
	var row wafCustomRuleRow
	err := s.DB.NewSelect().Model(&row).Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.WAFCustomRule{}, false, nil
		}
		return model.WAFCustomRule{}, false, err
	}
	return row.toModel(), true, nil
}

// UpdateWAFCustomRule overwrites a rule's editable fields; found=false when the
// id doesn't exist.
func (s *Store) UpdateWAFCustomRule(ctx context.Context, id string, in model.WAFCustomRule) (model.WAFCustomRule, bool, error) {
	row := wafCustomRuleRow{
		ID: id, Scope: orDefault(in.Scope, "global"), ServiceID: in.ServiceID,
		Name: in.Name, SecLang: in.SecLang, Enabled: in.Enabled, UpdatedAt: time.Now().UTC(),
	}
	res, err := s.DB.NewUpdate().Model(&row).
		Column("scope", "service_id", "name", "seclang", "enabled", "updated_at").
		Where("id = ?", id).Exec(ctx)
	if err != nil {
		return model.WAFCustomRule{}, false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.WAFCustomRule{}, false, nil
	}
	var out wafCustomRuleRow
	if err := s.DB.NewSelect().Model(&out).Where("id = ?", id).Scan(ctx); err != nil {
		return model.WAFCustomRule{}, false, err
	}
	return out.toModel(), true, nil
}

// DeleteWAFCustomRule removes a rule; found=false when the id doesn't exist.
func (s *Store) DeleteWAFCustomRule(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*wafCustomRuleRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
