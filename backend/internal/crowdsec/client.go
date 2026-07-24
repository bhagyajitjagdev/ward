// Package crowdsec is a thin read-only client for a CrowdSec LAPI. Ward observes
// decisions (to display them); enforcement is the Caddy bouncer's job at the edge.
// Ward is never in the request path (control plane, not data plane).
package crowdsec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Decision is one active CrowdSec ban/captcha, as returned by GET /v1/decisions.
type Decision struct {
	ID       int    `json:"id"`
	Origin   string `json:"origin"`   // crowdsec | cscli | lists:...
	Type     string `json:"type"`     // ban | captcha | throttle
	Scope    string `json:"scope"`    // Ip | Range | ...
	Value    string `json:"value"`    // the IP/CIDR
	Duration string `json:"duration"` // e.g. "3h59m12s"
	Scenario string `json:"scenario"` // what tripped it
}

// Client talks to a CrowdSec LAPI with a bouncer API key (read-only use here).
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds a client for baseURL (LAPI root, e.g. http://crowdsec:8080/) + apiKey.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Decisions returns the currently active decisions. A LAPI with none returns JSON
// null, which decodes to an empty slice.
func (c *Client) Decisions(ctx context.Context) ([]Decision, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/decisions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LAPI returned %s", resp.Status)
	}
	var out []Decision
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
