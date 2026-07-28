package edge

import (
	"os"
	"strings"
	"testing"
)

// TestVersionsMatchDockerfile guards against the embedded edge versions drifting from
// the pins in caddy/Dockerfile (the edge image) AND backend/Dockerfile (which builds
// the same plugin-enabled Caddy as the adapt binary). A bump must touch all three;
// Renovate's custom manager tracks both --with sets. Runs from the package dir, so the
// repo root is three up.
func TestVersionsMatchDockerfile(t *testing.T) {
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

	// Both Dockerfiles pin the same modules + the same Caddy base.
	for _, path := range []string{"../../../caddy/Dockerfile", "../../../backend/Dockerfile"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		df := string(b)
		for key, mod := range modules {
			ver := v[key]
			if ver == "" {
				t.Errorf("versions.json missing %q", key)
				continue
			}
			if !strings.Contains(df, mod+"@"+ver) {
				t.Errorf("%s does not pin %s@%s (versions.json %s=%s)", path, mod, ver, key, ver)
			}
		}
		if cv := v["caddy"]; cv == "" {
			t.Error("versions.json missing caddy")
		} else if want := "caddy:" + strings.TrimPrefix(cv, "v"); !strings.Contains(df, want) {
			t.Errorf("%s does not use base image %s", path, want)
		}
	}

	// The io.ward.* labels live only on the edge image.
	edge, err := os.ReadFile("../../../caddy/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	for key, ver := range v {
		if label := "io.ward." + key + `="` + ver + `"`; !strings.Contains(string(edge), label) {
			t.Errorf("caddy/Dockerfile missing/mismatched label %s", label)
		}
	}
}
