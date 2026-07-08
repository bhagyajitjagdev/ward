package waf

import (
	_ "embed"
	"testing"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

//go:embed testdata/sample-audit.json
var sampleAudit []byte

func TestParseAuditRecord(t *testing.T) {
	events, err := ParseAuditRecord(sampleAudit)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events (one per triggered rule), got %d", len(events))
	}

	byRule := map[int]model.WAFEvent{}
	for _, e := range events {
		byRule[e.RuleID] = e
	}

	// 942100 — the SQLi detection (a false-positive-shaped event)
	sqli, ok := byRule[942100]
	if !ok {
		t.Fatal("missing rule 942100")
	}
	if sqli.MatchedTarget != "ARGS:id" {
		t.Errorf("942100 matched_target = %q, want ARGS:id", sqli.MatchedTarget)
	}
	if sqli.MatchedValue != "1' OR '1'='1" {
		t.Errorf("942100 matched_value = %q", sqli.MatchedValue)
	}
	if sqli.Severity != "critical" {
		t.Errorf("942100 severity = %q, want critical", sqli.Severity)
	}
	if sqli.IsAnomalyScore {
		t.Error("942100 should not be flagged is_anomaly_score")
	}
	if sqli.Path != "/other" {
		t.Errorf("path = %q, want /other", sqli.Path)
	}
	if sqli.Host != "localhost" {
		t.Errorf("host = %q, want localhost (port stripped)", sqli.Host)
	}
	if sqli.EngineMode != "DetectionOnly" {
		t.Errorf("engine_mode = %q", sqli.EngineMode)
	}
	if sqli.Authed {
		t.Error("sample has no cookie/authorization header → authed should be false")
	}
	if !contains(sqli.Tags, "attack-sqli") {
		t.Errorf("942100 tags missing attack-sqli: %v", sqli.Tags)
	}

	// 949110 — anomaly aggregator (not a tuning target)
	anomaly, ok := byRule[949110]
	if !ok {
		t.Fatal("missing rule 949110")
	}
	if !anomaly.IsAnomalyScore {
		t.Error("949110 should be flagged is_anomaly_score")
	}
	if anomaly.MatchedTarget != "" {
		t.Errorf("949110 matched_target = %q, want empty", anomaly.MatchedTarget)
	}
}
