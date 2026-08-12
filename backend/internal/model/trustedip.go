package model

import "time"

// TrustedIP is an IP or CIDR that is exempt from every edge threat protection —
// the CrowdSec bouncer, the IP blocklist, the WAF, rate-limits, and geo blocking.
// It is global: a trusted address is trusted across every service. Authentication
// and the normal proxy still apply — "trusted" means "never blocked", not
// "unauthenticated".
type TrustedIP struct {
	ID        string    `json:"id"`
	CIDR      string    `json:"cidr"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
