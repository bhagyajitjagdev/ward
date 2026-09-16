package store

import (
	"context"
	"testing"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// TestSQLAggregation proves the dashboard rollups computed in SQL — including the
// dialect-specific epoch bucketing over bun's stored timestamp format.
func TestSQLAggregation(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC) // hour-aligned
	since := base.Add(-time.Minute)

	access := []model.AccessEvent{
		{TS: base.Add(5 * time.Minute), Host: "a", Path: "/", Status: 200, DurationMs: 10, Bytes: 100},
		{TS: base.Add(6 * time.Minute), Host: "a", Path: "/", Status: 302, DurationMs: 20, Bytes: 100},
		{TS: base.Add(65 * time.Minute), Host: "a", Path: "/x", Status: 404, DurationMs: 30, Bytes: 100},
		{TS: base.Add(66 * time.Minute), Host: "a", Path: "/x", Status: 503, DurationMs: 400, Bytes: 100},
		{TS: base.Add(-2 * time.Hour), Host: "a", Path: "/old", Status: 200, DurationMs: 1, Bytes: 1}, // outside the window
	}
	if err := st.InsertAccessEvents(ctx, access); err != nil {
		t.Fatal(err)
	}
	sum, err := st.AccessSummarySince(ctx, since, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 4 || sum.Bytes != 400 || sum.S2xx != 1 || sum.S3xx != 1 || sum.S4xx != 1 || sum.S5xx != 1 {
		t.Fatalf("summary = %+v", sum)
	}
	if sum.AvgMs != 115 {
		t.Fatalf("avg_ms = %v, want 115", sum.AvgMs)
	}
	p95, err := st.AccessP95Since(ctx, since, "", sum.Total)
	if err != nil || p95 != 400 {
		t.Fatalf("p95 = %v err=%v, want 400", p95, err)
	}
	series, err := st.AccessSeriesSince(ctx, since, "", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 2 || series[0].Bucket != base.Unix() || series[0].Requests != 2 || series[0].Errors != 0 ||
		series[1].Bucket != base.Add(time.Hour).Unix() || series[1].Requests != 2 || series[1].Errors != 1 {
		t.Fatalf("series = %+v (base=%d)", series, base.Unix())
	}

	svcID := "01a0abb5-0000-7000-8000-000000000001"
	if _, err := st.CreateService(ctx, model.Service{Name: "s", PublicHostnames: []string{"s.example.com"}, Upstreams: []string{"s:80"}}); err != nil {
		t.Fatal(err)
	}
	svcs, _ := st.ListServices(ctx)
	svcID = svcs[0].ID
	waf := []model.WAFEvent{
		{TxID: "1", TS: base.Add(1 * time.Minute), Path: "/", RuleID: 942100, ServiceID: &svcID},
		{TxID: "2", TS: base.Add(2 * time.Minute), Path: "/", RuleID: 942100, IsInterrupted: true, ServiceID: &svcID},
		{TxID: "2", TS: base.Add(2 * time.Minute), Path: "/", RuleID: 949110, IsInterrupted: true, IsAnomalyScore: true}, // aggregator: excluded
		{TxID: "3", TS: base.Add(61 * time.Minute), Path: "/", RuleID: 941100},
	}
	if err := st.InsertWAFEvents(ctx, waf); err != nil {
		t.Fatal(err)
	}
	ws, err := st.WAFSeriesSince(ctx, since, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 || ws[0].Bucket != base.Unix() || ws[0].Detections != 2 || ws[0].Blocked != 1 ||
		ws[1].Detections != 1 || ws[1].Blocked != 0 {
		t.Fatalf("waf series = %+v", ws)
	}
	bys, err := st.WAFCountByServiceSince(ctx, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(bys) != 1 || bys[0].ServiceID != svcID || bys[0].Count != 2 {
		t.Fatalf("by service = %+v", bys)
	}
}
