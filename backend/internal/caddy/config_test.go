package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func ptr(s string) *string { return &s }

func TestGenerate(t *testing.T) {
	services := []model.Service{
		{ID: "svc-n8n", Name: "n8n", PublicHostname: "n8n.example.com", Upstreams: []string{"n8n.private:5678"}, LBPolicy: "round_robin", Enabled: true, WAFEnabled: false},
		{ID: "svc-harbor", Name: "harbor", PublicHostname: "harbor.example.com", Upstreams: []string{"h1:80", "h2:80"}, LBPolicy: "least_conn", Enabled: true, WAFEnabled: true},
		{ID: "svc-off", Name: "off", PublicHostname: "off.example.com", Upstreams: []string{"x:1"}, Enabled: false},
		{ID: "svc-noup", Name: "noup", PublicHostname: "noup.example.com", Upstreams: nil, Enabled: true},
	}

	out, err := Generate(Input{Services: services}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}
	routes := cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["edge"].(map[string]any)["routes"].([]any)
	if len(routes) != 4 {
		t.Fatalf("want 4 routes (2 enabled services + 2 http->https redirects), got %d", len(routes))
	}
	s := string(out)
	for _, want := range []string{"n8n.example.com", "harbor.example.com", "least_conn", `"admin"`, `"reverse_proxy"`} {
		if !strings.Contains(s, want) {
			t.Errorf("generated config missing %q", want)
		}
	}
	for _, notWant := range []string{"off.example.com", "noup.example.com"} {
		if strings.Contains(s, notWant) {
			t.Errorf("generated config should not contain %q", notWant)
		}
	}
	if n := strings.Count(s, `"handler": "waf"`); n != 1 {
		t.Errorf("want exactly 1 waf handler, got %d", n)
	}
}

func TestGenerateWithExclusions(t *testing.T) {
	services := []model.Service{
		{ID: "svc-harbor", Name: "harbor", PublicHostname: "harbor.example.com", Upstreams: []string{"h:80"}, Enabled: true, WAFEnabled: true},
	}
	exclusions := []model.WAFExclusion{
		{Scope: "service", ServiceID: ptr("svc-harbor"), State: "active", SecLang: `SecRule REQUEST_URI "@beginsWith /v2/" "id:90000001,phase:1,pass,nolog,ctl:ruleRemoveTargetById=942100;ARGS:id"`},
		{Scope: "global", State: "active", SecLang: "SecRuleRemoveById 920350"},
		{Scope: "service", ServiceID: ptr("svc-harbor"), State: "draft", SecLang: "SecRuleRemoveById 999999"}, // not active → excluded
	}
	out, err := Generate(Input{Services: services, Exclusions: exclusions}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "ruleRemoveTargetById=942100;ARGS:id") {
		t.Error("service exclusion not composed into directives")
	}
	if !strings.Contains(s, "SecRuleRemoveById 920350") {
		t.Error("global exclusion not composed into directives")
	}
	if strings.Contains(s, "999999") {
		t.Error("non-active (draft) exclusion should not be composed")
	}
	// Exclusions must precede the CRS rules include: ctl:ruleRemoveById only
	// suppresses a rule that has not run yet, and Coraza evaluates in directive
	// order within a phase. (A no-op otherwise — the real-box wedge bug.)
	if excl, crs := strings.Index(s, "ruleRemoveTargetById=942100"), strings.Index(s, "Include @owasp_crs/*.conf"); excl < 0 || crs < 0 || excl > crs {
		t.Errorf("exclusions must come before the CRS include (exclusion idx %d, CRS include idx %d)", excl, crs)
	}
}

func TestGenerateWithCustomRules(t *testing.T) {
	services := []model.Service{
		{ID: "svc-a", Name: "a", PublicHostname: "a.example.com", Upstreams: []string{"a:80"}, Enabled: true, WAFEnabled: true},
		{ID: "svc-b", Name: "b", PublicHostname: "b.example.com", Upstreams: []string{"b:80"}, Enabled: true, WAFEnabled: true},
	}
	exclusions := []model.WAFExclusion{
		{Scope: "global", State: "active", SecLang: "SecRuleRemoveById 920350"},
	}
	rules := []model.WAFCustomRule{
		{Scope: "global", Enabled: true, SecLang: `SecRule REQUEST_METHOD "@streq TRACE" "id:1001,phase:1,deny,status:405"`},
		{Scope: "service", ServiceID: ptr("svc-a"), Enabled: true, SecLang: `SecRule ARGS:debug "@rx ^1$" "id:1002,phase:2,deny"`},
		{Scope: "global", Enabled: false, SecLang: "SecRuleRemoveById 888888"}, // disabled → excluded
	}
	out, err := Generate(Input{Services: services, Exclusions: exclusions, CustomRules: rules}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "id:1001") {
		t.Error("global custom rule not composed into directives")
	}
	if !strings.Contains(s, "id:1002") {
		t.Error("service custom rule not composed into directives")
	}
	if strings.Contains(s, "888888") {
		t.Error("disabled custom rule should not be composed")
	}
	// Deterministic slot: generated exclusions, then custom rules, then the CRS include.
	excl, rule, crs := strings.Index(s, "SecRuleRemoveById 920350"), strings.Index(s, "id:1001"), strings.Index(s, "Include @owasp_crs/*.conf")
	if excl < 0 || rule < 0 || crs < 0 || !(excl < rule && rule < crs) {
		t.Errorf("want exclusion < custom rule < CRS include, got idx %d / %d / %d", excl, rule, crs)
	}
	// The service-scoped rule must be in svc-a's directives only. Directives are
	// per-service strings; svc-b's must not contain id:1002.
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	raw := string(out)
	if got := strings.Count(raw, "id:1002"); got != 1 {
		t.Errorf("service-scoped custom rule appears %d times, want 1 (svc-a only)", got)
	}
	if got := strings.Count(raw, "id:1001"); got != 2 {
		t.Errorf("global custom rule appears %d times, want 2 (both services)", got)
	}
}

func TestGenerateWithCrowdSec(t *testing.T) {
	services := []model.Service{
		{ID: "s1", Name: "app", PublicHostname: "app.example.com", Upstreams: []string{"app:80"}, Enabled: true},
	}
	opt := DefaultOptions()
	opt.DisableAutoHTTPS = true

	// Disabled → no crowdsec app or handler.
	out, err := Generate(Input{Services: services}, opt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "crowdsec") {
		t.Error("crowdsec must be absent when disabled")
	}

	// Enabled → app present, and the bouncer is the very first route handler.
	opt.CrowdSecEnabled = true
	opt.CrowdSecAPIURL = "http://crowdsec:8080/"
	opt.CrowdSecAPIKey = "secret-key"
	out, err = Generate(Input{Services: services}, opt)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Apps struct {
			CrowdSec map[string]any `json:"crowdsec"`
			HTTP     struct {
				Servers struct {
					Edge struct {
						Routes []struct {
							Handle []struct {
								Handler string `json:"handler"`
							} `json:"handle"`
						} `json:"routes"`
					} `json:"edge"`
				} `json:"servers"`
			} `json:"http"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Apps.CrowdSec["api_key"] != "secret-key" || cfg.Apps.CrowdSec["enable_hard_fails"] != false {
		t.Errorf("crowdsec app not wired (fail-open expected): %#v", cfg.Apps.CrowdSec)
	}
	routes := cfg.Apps.HTTP.Servers.Edge.Routes
	if len(routes) == 0 || len(routes[0].Handle) == 0 || routes[0].Handle[0].Handler != "crowdsec" {
		t.Errorf("crowdsec must be the first route handler, got routes=%#v", routes)
	}
}

func TestGenerateExclusionSecLang(t *testing.T) {
	// prefix path + target → runtime rule with reserved id (backward-compatible form)
	got := GenerateExclusionSecLang(90000001, 942100, "prefix", "/leads/batch", nil, "ARGS:id")
	want := `SecRule REQUEST_URI "@beginsWith /leads/batch" "id:90000001,phase:1,pass,nolog,ctl:ruleRemoveTargetById=942100;ARGS:id"`
	if got != want {
		t.Errorf("prefix path+target:\n got %q\nwant %q", got, want)
	}
	// empty match-type defaults to prefix (existing rows / contextual quick-creates)
	if got := GenerateExclusionSecLang(90000001, 942100, "", "/x", nil, ""); got != `SecRule REQUEST_URI "@beginsWith /x" "id:90000001,phase:1,pass,nolog,ctl:ruleRemoveById=942100"` {
		t.Errorf("default match-type: got %q", got)
	}
	// exact + regex operators
	if got := GenerateExclusionSecLang(1, 942100, "exact", "/a", nil, ""); !strings.Contains(got, `"@streq /a"`) {
		t.Errorf("exact op: got %q", got)
	}
	if got := GenerateExclusionSecLang(1, 942100, "regex", "^/api/.*$", nil, ""); !strings.Contains(got, `"@rx ^/api/.*$"`) {
		t.Errorf("regex op: got %q", got)
	}
	// method-only → single SecRule on REQUEST_METHOD
	if got := GenerateExclusionSecLang(1, 942100, "prefix", "", []string{"POST", "PUT"}, ""); got != `SecRule REQUEST_METHOD "@rx ^(POST|PUT)$" "id:1,phase:1,pass,nolog,ctl:ruleRemoveById=942100"` {
		t.Errorf("method-only: got %q", got)
	}
	// path + methods → a SINGLE rule against REQUEST_LINE (not a chain — Coraza can't
	// gate a chain starter's ctl). prefix path is regex-escaped.
	got = GenerateExclusionSecLang(90000002, 942100, "prefix", "/api/leads", []string{"POST"}, "ARGS:id")
	wantLine := `SecRule REQUEST_LINE "@rx ^(POST)\s+/api/leads" "id:90000002,phase:1,pass,nolog,ctl:ruleRemoveTargetById=942100;ARGS:id"`
	if got != wantLine {
		t.Errorf("path+method:\n got %q\nwant %q", got, wantLine)
	}
	// regex path + methods → REQUEST_LINE with the URI-anchors stripped from the pattern
	if got := GenerateExclusionSecLang(3, 942100, "regex", "^/api/v[0-9]+/x$", []string{"GET"}, ""); got != `SecRule REQUEST_LINE "@rx ^(GET)\s+/api/v[0-9]+/x" "id:3,phase:1,pass,nolog,ctl:ruleRemoveById=942100"` {
		t.Errorf("regex+method: got %q", got)
	}
	// no path, with target → configure-time target removal
	if got := GenerateExclusionSecLang(0, 942100, "", "", nil, "ARGS:id"); got != `SecRuleUpdateTargetById 942100 "!ARGS:id"` {
		t.Errorf("no-path target: got %q", got)
	}
	// no path, no target, no methods → whole-rule removal
	if got := GenerateExclusionSecLang(0, 942100, "", "", nil, ""); got != "SecRuleRemoveById 942100" {
		t.Errorf("whole rule: got %q", got)
	}
}

func TestGenerateWithBlocks(t *testing.T) {
	services := []model.Service{
		{ID: "svc1", Name: "app", PublicHostname: "app.example.com", Upstreams: []string{"app:80"}, Enabled: true},
	}
	blocks := []model.BlockedIP{
		{Scope: "global", CIDR: "1.2.3.4"},
		{Scope: "global", CIDR: "10.0.0.0/8"},
		{Scope: "service", ServiceID: ptr("svc1"), CIDR: "5.6.7.8"},
	}
	out, err := Generate(Input{Services: services, Blocks: blocks}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	routes := cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["edge"].(map[string]any)["routes"].([]any)
	if len(routes) != 3 {
		t.Fatalf("want 3 routes (global-deny + http->https redirect + service), got %d", len(routes))
	}
	firstMatch := routes[0].(map[string]any)["match"].([]any)[0].(map[string]any)
	if _, ok := firstMatch["remote_ip"]; !ok {
		t.Error("first route should be a remote_ip deny")
	}
	s := string(out)
	for _, want := range []string{"1.2.3.4", "10.0.0.0/8", "5.6.7.8", `"static_response"`, `"403"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in generated config", want)
		}
	}
}

func TestGenerateTLS(t *testing.T) {
	services := []model.Service{
		{ID: "1", PublicHostname: "internal.example.com", Upstreams: []string{"x:1"}, Enabled: true, TLSMode: "internal"},
		{ID: "2", PublicHostname: "managed.example.com", Upstreams: []string{"x:1"}, Enabled: true, TLSMode: "managed"},
		{ID: "3", PublicHostname: "plain.example.com", Upstreams: []string{"x:1"}, Enabled: true, TLSMode: "none"},
	}
	opt := DefaultOptions()
	opt.ACMEEmail = "ops@example.com"
	out, err := Generate(Input{Services: services}, opt)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"module": "internal"`, "internal.example.com", `"module": "acme"`, "ops@example.com", `"skip"`, "plain.example.com", ":443", `"302"`, "disable_redirects"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS config missing %q", want)
		}
	}
	// the none-mode host must NOT get an http->https redirect
	if strings.Count(s, `"302"`) != 2 {
		t.Errorf("want 2 redirects (internal + managed, not the none host), got %d", strings.Count(s, `"302"`))
	}

	// dev-disabled → no HTTPS, auto-https off
	opt.DisableAutoHTTPS = true
	out2, _ := Generate(Input{Services: services}, opt)
	if !strings.Contains(string(out2), `"disable": true`) {
		t.Error("expected automatic_https disabled")
	}
	if strings.Contains(string(out2), `"tls"`) {
		t.Error("dev-disabled config should have no tls app")
	}
}
