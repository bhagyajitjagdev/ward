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

	out, err := Generate(services, nil, nil, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("generated config is not valid JSON: %v", err)
	}
	routes := cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["edge"].(map[string]any)["routes"].([]any)
	if len(routes) != 2 {
		t.Fatalf("want 2 routes (disabled + upstream-less excluded), got %d", len(routes))
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
	out, err := Generate(services, exclusions, nil, DefaultOptions())
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
}

func TestGenerateExclusionSecLang(t *testing.T) {
	// path + target → runtime rule with reserved id
	got := GenerateExclusionSecLang(90000001, 942100, "/leads/batch", "ARGS:id")
	want := `SecRule REQUEST_URI "@beginsWith /leads/batch" "id:90000001,phase:1,pass,nolog,ctl:ruleRemoveTargetById=942100;ARGS:id"`
	if got != want {
		t.Errorf("path+target:\n got %q\nwant %q", got, want)
	}
	// no path, with target → configure-time target removal
	if got := GenerateExclusionSecLang(0, 942100, "", "ARGS:id"); got != `SecRuleUpdateTargetById 942100 "!ARGS:id"` {
		t.Errorf("no-path target: got %q", got)
	}
	// no path, no target → whole-rule removal
	if got := GenerateExclusionSecLang(0, 942100, "", ""); got != "SecRuleRemoveById 942100" {
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
	out, err := Generate(services, nil, blocks, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	routes := cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["edge"].(map[string]any)["routes"].([]any)
	if len(routes) != 2 {
		t.Fatalf("want 2 routes (global-deny first + service), got %d", len(routes))
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
	out, err := Generate(services, nil, nil, opt)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"module": "internal"`, "internal.example.com", `"module": "acme"`, "ops@example.com", `"skip"`, "plain.example.com", ":443"} {
		if !strings.Contains(s, want) {
			t.Errorf("TLS config missing %q", want)
		}
	}

	// dev-disabled → no HTTPS, auto-https off
	opt.DisableAutoHTTPS = true
	out2, _ := Generate(services, nil, nil, opt)
	if !strings.Contains(string(out2), `"disable": true`) {
		t.Error("expected automatic_https disabled")
	}
	if strings.Contains(string(out2), `"tls"`) {
		t.Error("dev-disabled config should have no tls app")
	}
}
