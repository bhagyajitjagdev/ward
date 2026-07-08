package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type auditRow struct {
	bun.BaseModel `bun:"table:audit_log,alias:al"`

	ID        string    `bun:"id,pk"`
	Actor     string    `bun:"actor"`
	Action    string    `bun:"action,notnull"`
	Target    string    `bun:"target"`
	Detail    string    `bun:"detail"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

func (r auditRow) toModel() model.AuditEntry {
	return model.AuditEntry{ID: r.ID, Actor: r.Actor, Action: r.Action, Target: r.Target, Detail: r.Detail, CreatedAt: r.CreatedAt}
}

// WriteAudit appends an entry to the audit trail (best-effort; never blocks a request).
func (s *Store) WriteAudit(ctx context.Context, actor, action, target, detail string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	row := &auditRow{ID: id.String(), Actor: actor, Action: action, Target: target, Detail: detail, CreatedAt: time.Now().UTC()}
	_, err = s.DB.NewInsert().Model(row).Exec(ctx)
	return err
}

// ListAudit returns recent audit entries, newest first (default/max limit 100/500).
func (s *Store) ListAudit(ctx context.Context, limit int) ([]model.AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []auditRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.AuditEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}
