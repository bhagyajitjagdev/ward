package api

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// specDoc parses the embedded YAML through the same path the handler serves.
func specDoc(t *testing.T) map[string]any {
	t.Helper()
	b, err := specJSON()
	if err != nil {
		t.Fatalf("spec does not convert to JSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("served spec is not valid JSON: %v", err)
	}
	return doc
}

func TestSpecParses(t *testing.T) {
	doc := specDoc(t)
	if v, _ := doc["openapi"].(string); !strings.HasPrefix(v, "3.") {
		t.Fatalf("openapi version = %q, want 3.x", v)
	}
	if _, ok := doc["paths"].(map[string]any); !ok {
		t.Fatal("spec has no paths object")
	}
}

// TestSpecCoversRoutes extracts every mux pattern registered in Routes() from the
// api.go source and asserts the spec documents it — so adding a route without
// spec'ing it fails the build. (Source-scraping beats a hand-kept list: there's
// nothing to forget to update.)
func TestSpecCoversRoutes(t *testing.T) {
	doc := specDoc(t)
	paths, _ := doc["paths"].(map[string]any)

	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	re := regexp.MustCompile(`mux\.HandleFunc\("(GET|POST|PATCH|DELETE) ([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) < 40 {
		t.Fatalf("only found %d routes in api.go — the extraction regex is probably broken", len(matches))
	}
	for _, m := range matches {
		method, route := strings.ToLower(m[1]), m[2]
		p, ok := paths[route].(map[string]any)
		if !ok {
			t.Errorf("route %q is not in openapi.yaml", route)
			continue
		}
		if _, ok := p[method]; !ok {
			t.Errorf("route %q lacks a %s operation in openapi.yaml", route, strings.ToUpper(method))
		}
	}
}
