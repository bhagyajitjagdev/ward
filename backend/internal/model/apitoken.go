package model

import "time"

// APIToken is a long-lived, revocable bearer credential for scripting Ward. The
// raw Token is returned only once at creation; only its hash is stored.
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	UserID     *string    `json:"user_id,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Revoked    bool       `json:"revoked"`
	CreatedAt  time.Time  `json:"created_at"`
	Token      string     `json:"token,omitempty"` // populated only on creation
}
