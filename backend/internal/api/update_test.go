package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// testAPI stands up the real router over a temp SQLite store with no edge (applier
// nil → reconcile is a no-op) and returns a request helper bound to an owner session.
func testAPI(t *testing.T) func(method, path string, body any) (int, map[string]any) {
	t.Helper()
	st, err := store.Open("file:" + filepath.Join(t.TempDir(), "ward.db") + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	srv := httptest.NewServer(New(st, nil).Routes())
	t.Cleanup(srv.Close)

	var token string
	do := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		req, _ := http.NewRequest(method, srv.URL+path, &buf)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	st1, setup := do("POST", "/auth/setup", map[string]any{"username": "owner", "password": "supersecret1"})
	if st1 != 201 {
		t.Fatalf("setup → %d %v", st1, setup)
	}
	token, _ = setup["token"].(string)
	return do
}

func mustCreate(t *testing.T, do func(string, string, any) (int, map[string]any), path string, body any) string {
	t.Helper()
	st, out := do("POST", path, body)
	if st != 201 {
		t.Fatalf("POST %s → %d %v", path, st, out)
	}
	return out["id"].(string)
}

// TestPatchServiceIsPartial: a body with one field changes that field and nothing
// else — the WAF stays on and enforcing, auth stays, the service stays enabled.
func TestPatchServiceIsPartial(t *testing.T) {
	do := testAPI(t)
	id := mustCreate(t, do, "/services", map[string]any{
		"name": "app", "public_hostnames": []string{"app.example.com"}, "upstreams": []string{"10.0.0.5:8080"},
		"tls_mode": "none", "waf_enabled": true, "waf_mode": "On", "waf_skip_paths": []string{"/sse"},
		"lb_policy":    "least_conn",
		"http":         map[string]any{"basic_auth_user": "u", "basic_auth_password": "secretpw", "security_headers": true},
		"health_check": map[string]any{"active": true, "path": "/health"},
	})

	st, svc := do("PATCH", "/services/"+id, map[string]any{"name": "renamed"})
	if st != 200 {
		t.Fatalf("PATCH name → %d %v", st, svc)
	}
	http_ := svc["http"].(map[string]any)
	hc := svc["health_check"].(map[string]any)
	if svc["name"] != "renamed" || svc["waf_enabled"] != true || svc["waf_mode"] != "On" || svc["enabled"] != true ||
		svc["tls_mode"] != "none" || svc["lb_policy"] != "least_conn" ||
		http_["basic_auth_user"] != "u" || http_["security_headers"] != true || hc["active"] != true ||
		len(svc["waf_skip_paths"].([]any)) != 1 {
		t.Fatalf("renaming reset other fields: %v", svc)
	}

	// Nested merge: one http field changes, the others (incl. the stored password hash) stay.
	st, svc = do("PATCH", "/services/"+id, map[string]any{"http": map[string]any{"compression": true}})
	http_ = svc["http"].(map[string]any)
	if st != 200 || http_["compression"] != true || http_["basic_auth_user"] != "u" || http_["security_headers"] != true {
		t.Fatalf("nested merge broke http: %d %v", st, svc)
	}
	if _, leaked := http_["basic_auth_hash"]; leaked {
		t.Fatal("basic_auth_hash must never leave the API")
	}

	// null clears; arrays replace as a whole.
	st, svc = do("PATCH", "/services/"+id, map[string]any{"waf_skip_paths": nil, "waf_mode": nil})
	if st != 200 || len(svc["waf_skip_paths"].([]any)) != 0 || svc["waf_mode"] != "" || svc["waf_enabled"] != true {
		t.Fatalf("null should clear just those fields: %d %v", st, svc)
	}

	// The single-hostname alias on its own replaces the list.
	st, svc = do("PATCH", "/services/"+id, map[string]any{"public_hostname": "b.example.com"})
	if st != 200 || svc["public_hostname"] != "b.example.com" || len(svc["public_hostnames"].([]any)) != 1 {
		t.Fatalf("alias patch: %d %v", st, svc)
	}

	// Explicitly turning basic auth off works; the WAF is still untouched.
	st, svc = do("PATCH", "/services/"+id, map[string]any{"http": map[string]any{"basic_auth_user": ""}})
	http_ = svc["http"].(map[string]any)
	if st != 200 || http_["basic_auth_user"] != nil || http_["compression"] != true || svc["waf_enabled"] != true {
		t.Fatalf("clearing basic auth: %d %v", st, svc)
	}

	// Validation still runs on the merged result.
	if st, out := do("PATCH", "/services/"+id, map[string]any{"tls_mode": "bogus"}); st != 400 {
		t.Fatalf("invalid merged value should 400, got %d %v", st, out)
	}
	if st, out := do("PATCH", "/services/"+id, []string{"not", "an", "object"}); st != 400 {
		t.Fatalf("non-object body should 400, got %d %v", st, out)
	}
	if st, _ := do("PATCH", "/services/nope", map[string]any{"name": "x"}); st != 404 {
		t.Fatalf("unknown id should 404, got %d", st)
	}
}

func TestPatchRulesArePartial(t *testing.T) {
	do := testAPI(t)
	sid := mustCreate(t, do, "/services", map[string]any{
		"name": "app", "public_hostnames": []string{"app.example.com"}, "upstreams": []string{"10.0.0.5:8080"}, "tls_mode": "none",
	})

	// Blocklist: an allow-only, per-service, temporary entry keeps all of that when only the reason changes.
	bid := mustCreate(t, do, "/blocklist", map[string]any{
		"cidr": "203.0.113.0/24", "mode": "allow", "scope": "service", "service_id": sid, "reason": "office", "expires_at": "2026-12-31T00:00:00Z",
	})
	st, b := do("PATCH", "/blocklist/"+bid, map[string]any{"reason": "office wifi"})
	if st != 200 || b["mode"] != "allow" || b["scope"] != "service" || b["service_id"] != sid || b["expires_at"] != "2026-12-31T00:00:00Z" || b["reason"] != "office wifi" {
		t.Fatalf("block patch reset fields: %d %v", st, b)
	}
	st, b = do("PATCH", "/blocklist/"+bid, map[string]any{"expires_at": nil})
	if st != 200 || b["expires_at"] != nil || b["mode"] != "allow" {
		t.Fatalf("null expiry should make it permanent and change nothing else: %d %v", st, b)
	}
	st, b = do("PATCH", "/blocklist/"+bid, map[string]any{"scope": "global"})
	if st != 200 || b["scope"] != "global" || b["service_id"] != nil {
		t.Fatalf("scope→global should drop service_id: %d %v", st, b)
	}
	if st, out := do("PATCH", "/blocklist/"+bid, map[string]any{"scope": "service"}); st != 400 {
		t.Fatalf("scope=service without service_id should 400, got %d %v", st, out)
	}

	// Geo: allow-only + per-service survives a countries-only patch (before: flipped to a global block).
	gid := mustCreate(t, do, "/geo-rules", map[string]any{"countries": []string{"IN", "DE"}, "mode": "allow", "scope": "service", "service_id": sid})
	st, g := do("PATCH", "/geo-rules/"+gid, map[string]any{"countries": []string{"in"}})
	if st != 200 || g["mode"] != "allow" || g["scope"] != "service" || g["service_id"] != sid || len(g["countries"].([]any)) != 1 || g["countries"].([]any)[0] != "IN" {
		t.Fatalf("geo patch reset fields: %d %v", st, g)
	}

	// Rate limit: per-service scope survives a max_events-only patch (before: became global).
	rid := mustCreate(t, do, "/rate-limits", map[string]any{"max_events": 100, "window": "1m", "scope": "service", "service_id": sid})
	st, rl := do("PATCH", "/rate-limits/"+rid, map[string]any{"max_events": 50})
	if st != 200 || rl["scope"] != "service" || rl["service_id"] != sid || rl["window"] != "1m" || rl["max_events"] != float64(50) {
		t.Fatalf("rate-limit patch reset fields: %d %v", st, rl)
	}
	if st, out := do("PATCH", "/rate-limits/"+rid, map[string]any{"window": "soon"}); st != 400 {
		t.Fatalf("invalid window should 400, got %d %v", st, out)
	}

	// Custom rule: per-service + disabled survives a name-only patch (before: became global).
	cid := mustCreate(t, do, "/waf-custom-rules", map[string]any{
		"name": "no-trace", "seclang": `SecRule REQUEST_METHOD "@streq TRACE" "id:90001,phase:1,deny"`, "scope": "service", "service_id": sid, "enabled": false,
	})
	st, cr := do("PATCH", "/waf-custom-rules/"+cid, map[string]any{"name": "block-trace"})
	if st != 200 || cr["name"] != "block-trace" || cr["scope"] != "service" || cr["service_id"] != sid || cr["enabled"] != false {
		t.Fatalf("custom-rule patch reset fields: %d %v", st, cr)
	}
	st, cr = do("PATCH", "/waf-custom-rules/"+cid, map[string]any{"enabled": true})
	if st != 200 || cr["enabled"] != true || cr["name"] != "block-trace" {
		t.Fatalf("enabling should change only enabled: %d %v", st, cr)
	}
}
