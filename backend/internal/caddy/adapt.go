package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AdaptFragment converts a per-service Caddyfile fragment (raw directives) into the
// Caddy route objects to splice into the service's subroute. It wraps the fragment
// in a throwaway site block, runs `caddy adapt`, and returns the resulting routes.
// A syntax error surfaces as the error; the subsequent config load provisions them.
//
// Requires the `caddy` binary on PATH (present in the ward image). An empty fragment
// returns (nil, nil).
func AdaptFragment(raw string) ([]json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	f, err := os.CreateTemp("", "ward-adapt-*.caddyfile")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(":0 {\n" + raw + "\n}\n"); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	cmd := exec.Command("caddy", "adapt", "--config", f.Name(), "--adapter", "caddyfile")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errb.String()); msg != "" {
			// Hide the throwaway temp path; keep just "line N: …". The wrapper adds one
			// line (the site-block opener), so the reported number is off by one.
			msg = strings.ReplaceAll(msg, f.Name()+":", "line ")
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("caddy adapt: %w", err)
	}

	var c struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Routes []json.RawMessage `json:"routes"`
				} `json:"servers"`
			} `json:"http"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(out.Bytes(), &c); err != nil {
		return nil, err
	}
	for _, s := range c.Apps.HTTP.Servers { // the single throwaway server
		return s.Routes, nil
	}
	return nil, nil
}
