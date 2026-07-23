package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "ward.db") + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	st, err := Open(dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestPruneWAFEvents(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	events := []model.WAFEvent{
		{TxID: "old", TS: now.AddDate(0, 0, -40), Path: "/old", RuleID: 942100},
		{TxID: "edge", TS: now.AddDate(0, 0, -20), Path: "/edge", RuleID: 942100},
		{TxID: "new", TS: now, Path: "/new", RuleID: 942100},
	}
	if err := st.InsertWAFEvents(ctx, events); err != nil {
		t.Fatalf("insert: %v", err)
	}

	n, err := st.PruneWAFEvents(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("pruned %d rows, want 1", n)
	}

	left, err := st.ListWAFEvents(ctx, WAFEventFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(left) != 2 {
		t.Fatalf("%d events left, want 2", len(left))
	}
	for _, e := range left {
		if e.TxID == "old" {
			t.Fatalf("the old event survived the prune")
		}
	}
}

func TestPruneAccessEvents(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	events := []model.AccessEvent{
		{TS: now.AddDate(0, 0, -10), Host: "a", Path: "/old", Status: 200},
		{TS: now, Host: "a", Path: "/new", Status: 200},
	}
	if err := st.InsertAccessEvents(ctx, events); err != nil {
		t.Fatalf("insert: %v", err)
	}

	n, err := st.PruneAccessEvents(ctx, now.AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("pruned %d rows, want 1", n)
	}
}

func TestWAFRetentionDaysSetting(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if got := st.WAFRetentionDays(ctx, 30); got != 30 {
		t.Fatalf("unset: got %d, want the 30 fallback", got)
	}
	if err := st.SetSetting(ctx, WAFRetentionKey, "90"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := st.WAFRetentionDays(ctx, 30); got != 90 {
		t.Fatalf("set: got %d, want 90", got)
	}
	// Garbage values fall back rather than breaking the pruner.
	if err := st.SetSetting(ctx, WAFRetentionKey, "not-a-number"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := st.WAFRetentionDays(ctx, 30); got != 30 {
		t.Fatalf("garbage: got %d, want the 30 fallback", got)
	}
}
