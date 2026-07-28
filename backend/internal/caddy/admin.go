package caddy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Caddy admin API. Caddy's admin endpoint is unauthenticated,
// so it must be bound to an internal-only network — Ward is its only client.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a Caddy admin client for e.g. "http://localhost:2019".
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		// Generous ceiling: a config /load can legitimately take many seconds (Caddy
		// tears down old listeners synchronously). A too-tight timeout cancels the load
		// mid-reload, which can wedge Caddy's admin endpoint — far worse than waiting.
		// The caller's context still bounds it (2 min for API pushes).
		http: &http.Client{Timeout: 2 * time.Minute},
	}
}

// Load pushes a full config to Caddy (POST /load). Caddy validates and applies
// it atomically; on failure it keeps the previous config untouched.
func (c *Client) Load(ctx context.Context, cfgJSON []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/load", bytes.NewReader(cfgJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("caddy load: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("caddy load rejected (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
