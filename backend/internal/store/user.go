package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/auth"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type userRow struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID           string    `bun:"id,pk"`
	Username     string    `bun:"username,notnull"`
	PasswordHash string    `bun:"password_hash,notnull"`
	Role         string    `bun:"role,notnull"`
	IsOwner      bool      `bun:"is_owner,notnull"`
	CreatedAt    time.Time `bun:"created_at,notnull"`
}

func (r userRow) toModel() model.User {
	return model.User{ID: r.ID, Username: r.Username, Role: r.Role, IsOwner: r.IsOwner, CreatedAt: r.CreatedAt}
}

type sessionRow struct {
	bun.BaseModel `bun:"table:sessions,alias:se"`

	ID        string    `bun:"id,pk"`
	UserID    string    `bun:"user_id,notnull"`
	TokenHash string    `bun:"token_hash,notnull"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

// CountUsers returns the number of accounts (0 → setup not done).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	return s.DB.NewSelect().Model((*userRow)(nil)).Count(ctx)
}

// CreateUser inserts an account with an already-hashed password.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string, isOwner bool) (model.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.User{}, err
	}
	row := userRow{
		ID: id.String(), Username: username, PasswordHash: passwordHash,
		Role: orDefault(role, "admin"), IsOwner: isOwner, CreatedAt: time.Now().UTC(),
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		if isUniqueViolation(err) {
			return model.User{}, ErrConflict
		}
		return model.User{}, err
	}
	return row.toModel(), nil
}

// AuthenticateUser verifies credentials; returns nil (no error) on bad login.
func (s *Store) AuthenticateUser(ctx context.Context, username, password string) (*model.User, error) {
	var row userRow
	err := s.DB.NewSelect().Model(&row).Where("username = ?", username).Limit(1).Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !auth.CheckPassword(row.PasswordHash, password) {
		return nil, nil
	}
	m := row.toModel()
	return &m, nil
}

// ListUsers returns all accounts, newest first.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	var rows []userRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.User, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// GetUser returns a single account by id (nil if not found).
func (s *Store) GetUser(ctx context.Context, id string) (*model.User, error) {
	var row userRow
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

// DeleteUser removes an account (cascades its sessions); reports whether found.
func (s *Store) DeleteUser(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*userRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CreateSession stores a session token hash and returns its expiry.
func (s *Store) CreateSession(ctx context.Context, userID, tokenHash string, ttl time.Duration) (time.Time, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return time.Time{}, err
	}
	exp := time.Now().UTC().Add(ttl)
	row := sessionRow{ID: id.String(), UserID: userID, TokenHash: tokenHash, ExpiresAt: exp, CreatedAt: time.Now().UTC()}
	_, err = s.DB.NewInsert().Model(&row).Exec(ctx)
	return exp, err
}

// UserFromSession validates a session token hash and returns its user (nil if
// invalid or expired).
func (s *Store) UserFromSession(ctx context.Context, tokenHash string) (*model.User, error) {
	var sess sessionRow
	err := s.DB.NewSelect().Model(&sess).Where("token_hash = ?", tokenHash).Limit(1).Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, nil
	}
	return s.GetUser(ctx, sess.UserID)
}

// DeleteSession revokes a session by its token hash (logout).
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.DB.NewDelete().Model((*sessionRow)(nil)).Where("token_hash = ?", tokenHash).Exec(ctx)
	return err
}
