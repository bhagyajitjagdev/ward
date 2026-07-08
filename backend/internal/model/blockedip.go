package model

import "time"

// BlockedIP is a denied IP or CIDR. Global blocks apply to the whole edge;
// service blocks apply to one service. An expiry makes it a temporary ban.
type BlockedIP struct {
	ID        string     `json:"id"`
	Scope     string     `json:"scope"` // "global" | "service"
	Mode      string     `json:"mode"`  // "block" (deny these) | "allow" (deny everything else)
	ServiceID *string    `json:"service_id,omitempty"`
	CIDR      string     `json:"cidr"`
	Reason    string     `json:"reason,omitempty"`
	Source    string     `json:"source"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
