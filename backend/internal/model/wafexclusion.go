package model

import "time"

// WAFExclusion is a scoped rule exclusion. The operator picks a detection
// (rule + path + target) and Ward generates the SecLang; it composes into the
// generated Coraza directives (global → all WAF services, service → one).
type WAFExclusion struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // "global" | "service"
	ServiceID *string   `json:"service_id,omitempty"`
	RuleID    int       `json:"rule_id"`
	Path      string    `json:"path,omitempty"`   // path prefix; empty = whole handler
	Target    string    `json:"target,omitempty"` // e.g. "ARGS:id"; empty = whole rule
	SecLang   string    `json:"seclang"`
	State     string    `json:"state"`  // "active" (soak flow deferred)
	Source    string    `json:"source"` // "manual"
	CreatedAt time.Time `json:"created_at"`
}
