package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func sp(s string) *string { return &s }

// TestDesiredStateRoundTrip proves the snapshot/rollback contract: capture the
// state, change everything, restore, and the DB is back to the capture — ids and
// the FK mapping of WAF events to services included.
func TestDesiredStateRoundTrip(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	svc, err := st.CreateService(ctx, model.Service{
		Name: "app", PublicHostnames: []string{"app.example.com"}, Upstreams: []string{"app:80"},
		WAFEnabled: true, TLSMode: "none",
		HTTP: model.HTTPConfig{BasicAuthUser: "u", BasicAuthHash: "$2a$hash"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateExclusion(ctx, model.WAFExclusion{Scope: "service", ServiceID: sp(svc.ID), RuleID: 942100, Path: "/x", SecLang: "SecRuleRemoveById 942100", Methods: []string{"GET", "POST"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateWAFCustomRule(ctx, model.WAFCustomRule{Name: "r", SecLang: `SecRule REQUEST_METHOD "@streq TRACE" "id:90001,phase:1,deny"`, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	exp := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if _, err := st.CreateBlock(ctx, model.BlockedIP{CIDR: "10.0.0.1", ExpiresAt: &exp, Reason: "temp"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateRateLimit(ctx, model.RateLimit{MaxEvents: 10, Window: "1m"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateGeoRule(ctx, model.GeoRule{Countries: []string{"RU"}, Mode: "block"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateTrusted(ctx, model.TrustedIP{CIDR: "192.168.0.0/16", Note: "lan"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, WAFModeKey, "On"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, AccessRetentionKey, "3"); err != nil { // operational: must survive a rollback untouched
		t.Fatal(err)
	}
	// A detection mapped to the service — its FK must survive the restore.
	if err := st.InsertWAFEvents(ctx, []model.WAFEvent{{TxID: "t1", TS: time.Now().UTC(), Path: "/x", RuleID: 942100, ServiceID: sp(svc.ID)}}); err != nil {
		t.Fatal(err)
	}

	captured, err := st.LoadDesiredState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(captured)
	if err != nil {
		t.Fatal(err)
	}

	// Now wreck everything: mangle the service in place, add strangers, flip settings.
	// (A service *deleted* after the snapshot has already had its events unmapped by
	// the cascade — that's the delete's doing; a restore re-creates the row but can't
	// un-null what's gone. The common case — a bad edit — keeps the row and the FK.)
	mangled := svc
	mangled.Name, mangled.Upstreams, mangled.WAFEnabled = "mangled", []string{"wrong:1"}, false
	mangled.HTTP = model.HTTPConfig{}
	if _, err := st.UpdateService(ctx, svc.ID, mangled); err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateService(ctx, model.Service{Name: "other", PublicHostnames: []string{"other.example.com"}, Upstreams: []string{"o:80"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateBlock(ctx, model.BlockedIP{CIDR: "10.9.9.9"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, WAFModeKey, "DetectionOnly"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, TLSMinVersionKey, "1.3"); err != nil { // unset in the capture → must be cleared
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, AccessRetentionKey, "9"); err != nil {
		t.Fatal(err)
	}

	ds, err := ParseDesiredState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RestoreDesiredState(ctx, ds); err != nil {
		t.Fatalf("restore: %v", err)
	}

	after, err := st.LoadDesiredState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	afterRaw, _ := json.Marshal(after)
	if string(afterRaw) != string(raw) {
		t.Fatalf("state after restore differs from the capture\n got: %s\nwant: %s", afterRaw, raw)
	}
	if _, err := st.GetService(ctx, other.ID); err != ErrNotFound {
		t.Fatalf("stranger service should be pruned, got err=%v", err)
	}
	got, err := st.GetService(ctx, svc.ID)
	if err != nil {
		t.Fatalf("restored service missing: %v", err)
	}
	if got.Name != "app" || got.HTTP.BasicAuthHash != "$2a$hash" || !got.WAFEnabled || got.Upstreams[0] != "app:80" || got.CreatedAt.IsZero() {
		t.Fatalf("restored service lost fields: %+v", got)
	}
	if v, _ := st.GetSetting(ctx, TLSMinVersionKey); v != "" {
		t.Fatalf("setting unset in the capture should be cleared, got %q", v)
	}
	if v, _ := st.GetSetting(ctx, AccessRetentionKey); v != "9" {
		t.Fatalf("operational setting must not be touched by a restore, got %q", v)
	}
	// The detection kept its service mapping because the service row was upserted, not recreated.
	evs, err := st.ListWAFEvents(ctx, WAFEventFilter{ServiceID: svc.ID})
	if err != nil || len(evs) != 1 {
		t.Fatalf("waf event lost its service mapping: n=%d err=%v", len(evs), err)
	}
}

// TestSaveSnapshotDedupes: the reconciler re-applies every minute; identical
// applies must not add history rows.
func TestSaveSnapshotDedupes(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := st.SaveSnapshot(ctx, []byte(`{"a":1}`), []byte(`{"s":[]}`), ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SaveSnapshot(ctx, []byte(`{"a":2}`), []byte(`{"s":[]}`), "changed"); err != nil {
		t.Fatal(err)
	}
	snaps, err := st.ListSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 snapshots (3 identical collapse to 1), got %d", len(snaps))
	}
	if !snaps[0].Active || snaps[0].Note != "changed" || !snaps[0].Restorable {
		t.Fatalf("newest should be active + restorable + noted: %+v", snaps[0])
	}
	if snaps[1].Active {
		t.Fatal("older snapshot should be inactive")
	}
	full, err := st.GetSnapshot(ctx, snaps[0].ID)
	if err != nil || full == nil || full.CaddyJSON != `{"a":2}` || full.WardJSON != `{"s":[]}` {
		t.Fatalf("GetSnapshot: %+v err=%v", full, err)
	}
}
