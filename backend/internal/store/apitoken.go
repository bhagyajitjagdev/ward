package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type apiTokenRow struct {
	bun.BaseModel `bun:"table:api_tokens,alias:at"`

	ID         string     `bun:"id,pk"`
	Name       string     `bun:"name,notnull"`
	TokenHash  string     `bun:"token_hash,notnull"`
	UserID     *string    `bun:"user_id"`
	LastUsedAt *time.Time `bun:"last_used_at"`
	ExpiresAt  *time.Time `bun:"expires_at"`
	Revoked    bool       `bun:"revoked,notnull"`
	CreatedAt  time.Time  `bun:"created_at,notnull"`
}

func (r apiTokenRow) toModel() model.APIToken {
	return model.APIToken{
		ID: r.ID, Name: r.Name, UserID: r.UserID, LastUsedAt: r.LastUsedAt,
		ExpiresAt: r.ExpiresAt, Revoked: r.Revoked, CreatedAt: r.CreatedAt,
	}
}

// CreateAPIToken stores a token (by hash) owned by a user.
func (s *Store) CreateAPIToken(ctx context.Context, name, userID, tokenHash string, expiresAt *time.Time) (model.APIToken, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.APIToken{}, err
	}
	row := apiTokenRow{
		ID: id.String(), Name: name, TokenHash: tokenHash, UserID: &userID,
		ExpiresAt: expiresAt, Revoked: false, CreatedAt: time.Now().UTC(),
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		return model.APIToken{}, err
	}
	return row.toModel(), nil
}

// ListAPITokens returns all tokens (no secrets), newest first.
func (s *Store) ListAPITokens(ctx context.Context) ([]model.APIToken, error) {
	var rows []apiTokenRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.APIToken, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// RevokeAPIToken marks a token revoked; reports whether one was found.
func (s *Store) RevokeAPIToken(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewUpdate().Model((*apiTokenRow)(nil)).
		Set("revoked = ?", true).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UserFromBearer resolves a bearer token hash to a user, trying sessions first
// then API tokens. Used by the auth middleware.
func (s *Store) UserFromBearer(ctx context.Context, tokenHash string) (*model.User, error) {
	if u, err := s.UserFromSession(ctx, tokenHash); err != nil || u != nil {
		return u, err
	}
	return s.userFromAPIToken(ctx, tokenHash)
}

func (s *Store) userFromAPIToken(ctx context.Context, tokenHash string) (*model.User, error) {
	var row apiTokenRow
	err := s.DB.NewSelect().Model(&row).
		Where("token_hash = ? AND revoked = ?", tokenHash, false).Limit(1).Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.ExpiresAt != nil && time.Now().UTC().After(*row.ExpiresAt) {
		return nil, nil
	}
	if row.UserID == nil {
		return nil, nil
	}
	// best-effort last-used timestamp
	_, _ = s.DB.NewUpdate().Model((*apiTokenRow)(nil)).
		Set("last_used_at = ?", time.Now().UTC()).Where("id = ?", row.ID).Exec(ctx)
	return s.GetUser(ctx, *row.UserID)
}
