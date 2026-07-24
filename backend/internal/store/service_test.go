package store

import (
	"context"
	"testing"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func TestServiceMultiHostname(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	svc, err := st.CreateService(ctx, model.Service{
		Name:            "api",
		PublicHostnames: []string{"api.example.com", "api.svc.example.com"},
		Upstreams:       []string{"api:80"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(svc.PublicHostnames) != 2 || svc.PublicHostname != "api.example.com" {
		t.Fatalf("round-trip: primary=%q list=%v", svc.PublicHostname, svc.PublicHostnames)
	}

	// Both the primary and the extra resolve to the service; an unknown host is nil.
	for _, h := range []string{"api.example.com", "api.svc.example.com"} {
		id, err := st.GetServiceIDByHost(ctx, h)
		if err != nil {
			t.Fatal(err)
		}
		if id == nil || *id != svc.ID {
			t.Errorf("GetServiceIDByHost(%q) = %v, want %s", h, id, svc.ID)
		}
	}
	if id, _ := st.GetServiceIDByHost(ctx, "nope.example.com"); id != nil {
		t.Errorf("unknown host should be nil, got %v", id)
	}

	// Uniqueness spans the full set; excluding the owner clears it.
	dup, err := st.HostnamesInUse(ctx, []string{"api.svc.example.com", "fresh.example.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(dup) != 1 || dup[0] != "api.svc.example.com" {
		t.Errorf("HostnamesInUse = %v, want [api.svc.example.com]", dup)
	}
	if dup, _ := st.HostnamesInUse(ctx, []string{"api.svc.example.com"}, svc.ID); len(dup) != 0 {
		t.Errorf("excluding owner: want none, got %v", dup)
	}

	// Dropping the extra removes its host→service mapping.
	svc.PublicHostnames = []string{"api.example.com"}
	if _, err := st.UpdateService(ctx, svc.ID, svc); err != nil {
		t.Fatal(err)
	}
	if id, _ := st.GetServiceIDByHost(ctx, "api.svc.example.com"); id != nil {
		t.Errorf("dropped extra should not resolve, got %v", id)
	}
}
