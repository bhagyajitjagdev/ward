package edge

import (
	"os"
	"strings"
	"testing"
)

// TestVersionsMatchDockerfile guards against the embedded edge versions drifting from
// what caddy/Dockerfile actually pins and labels — a bump must touch both (Renovate
// updates both in one PR). Runs from the package dir, so the repo root is three up.
func TestVersionsMatchDockerfile(t *testing.T) {
	b, err := os.ReadFile("../../../caddy/Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	df := string(b)
	v := Versions()

	// component key -> the Go module path pinned via `--with …@version`.
	modules := map[string]string{
		"coraza_caddy":        "github.com/corazawaf/coraza-caddy/v2",
		"coraza":              "github.com/corazawaf/coraza/v3",
		"crs":                 "github.com/corazawaf/coraza-coreruleset/v4",
		"ratelimit":           "github.com/mholt/caddy-ratelimit",
		"maxmind_geolocation": "github.com/porech/caddy-maxmind-geolocation",
		"crowdsec_bouncer":    "github.com/hslatman/caddy-crowdsec-bouncer/http",
	}
	for key, mod := range modules {
		ver := v[key]
		if ver == "" {
			t.Errorf("versions.json missing %q", key)
			continue
		}
		if !strings.Contains(df, mod+"@"+ver) {
			t.Errorf("Dockerfile does not pin %s@%s (versions.json %s=%s)", mod, ver, key, ver)
		}
	}

	// Caddy is pinned by the base-image tag, not a --with line.
	if cv := v["caddy"]; cv == "" {
		t.Error("versions.json missing caddy")
	} else if want := "FROM caddy:" + strings.TrimPrefix(cv, "v"); !strings.Contains(df, want) {
		t.Errorf("Dockerfile base image is not %q (versions.json caddy=%s)", want, cv)
	}

	// Every component must also carry a matching io.ward.* label.
	for key, ver := range v {
		if label := "io.ward." + key + `="` + ver + `"`; !strings.Contains(df, label) {
			t.Errorf("Dockerfile missing/mismatched label %s", label)
		}
	}
}
