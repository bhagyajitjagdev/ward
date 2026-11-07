package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// An event whose request data carries a NUL byte (e.g. `?q=<script>%00`, decoded
// by the WAF into the matched value) must still be stored — bun refuses NUL bytes
// outright, and the ingesters drop a row they can't store, which would let a
// request hide itself from the event log just by including %00.

func TestWAFEventWithNULIsStored(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	ev := model.WAFEvent{
		TxID: "nul", TS: time.Now().UTC(), Path: "/q\x00", URI: "/q?q=<script>%00",
		RuleID: 941100, MatchedTarget: "ARGS:q", MatchedValue: "<script>\x00",
		Raw: "Matched Data: <script>\x00 found within ARGS:q", UserAgent: "ua\x00",
	}
	if err := st.InsertWAFEvents(ctx, []model.WAFEvent{ev}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := st.ListWAFEvents(ctx, WAFEventFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d events stored, want 1", len(got))
	}
	e := got[0]
	for name, v := range map[string]string{"path": e.Path, "matched_value": e.MatchedValue, "raw": e.Raw, "user_agent": e.UserAgent} {
		if strings.IndexByte(v, 0) >= 0 {
			t.Errorf("%s still holds a NUL byte: %q", name, v)
		}
		if !strings.Contains(v, `\x00`) {
			t.Errorf("%s lost the NUL instead of showing it as \\x00: %q", name, v)
		}
	}
}

func TestAccessEventWithNULIsStored(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	ev := model.AccessEvent{
		TS: time.Now().UTC(), Host: "a.test", ClientIP: "192.0.2.1", Method: "GET",
		Path: "/p\x00", Query: "q=\x00", Status: 200, UserAgent: "ua\x00",
	}
	if err := st.InsertAccessEvents(ctx, []model.AccessEvent{ev}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := st.ListAccessEvents(ctx, AccessFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d events stored, want 1", len(got))
	}
	if e := got[0]; e.Path != `/p\x00` || e.Query != `q=\x00` || e.UserAgent != `ua\x00` {
		t.Errorf("got path=%q query=%q ua=%q, want the NUL shown as \\x00", e.Path, e.Query, e.UserAgent)
	}
}
