package model

import "time"

// WAFCustomRule is user-authored raw SecLang, injected into the WAF directives
// (before-CRS slot, after the generated exclusions). The advanced escape hatch:
// Ward validates it by pushing the regenerated config to the edge, but the
// content is the operator's own — it can strengthen or weaken their WAF.
type WAFCustomRule struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // "global" | "service"
	ServiceID *string   `json:"service_id,omitempty"`
	Name      string    `json:"name"`
	SecLang   string    `json:"seclang"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
