package api

import (
	"strings"
	"testing"
)

type patchDoc struct {
	Name    string            `json:"name"`
	Count   int               `json:"count"`
	Tags    []string          `json:"tags"`
	Note    *string           `json:"note,omitempty"`
	Nested  patchNested       `json:"nested"`
	Labels  map[string]string `json:"labels,omitempty"`
	Big     int64             `json:"big"`
	Enabled bool              `json:"enabled"`
}

type patchNested struct {
	A string `json:"a,omitempty"`
	B string `json:"b,omitempty"`
	N int    `json:"n,omitempty"`
}

func TestMergePatchSemantics(t *testing.T) {
	note := "keep me"
	existing := patchDoc{
		Name: "orig", Count: 3, Tags: []string{"x", "y"}, Note: &note,
		Nested: patchNested{A: "a", B: "b", N: 7}, Labels: map[string]string{"k": "v"},
		Big: 9007199254740993, Enabled: true, // 2^53+1: must survive the merge exactly
	}
	body := `{"name":"new","nested":{"b":"B2","n":null},"tags":["only"],"note":null,"labels":{"k2":"v2"}}`
	p, err := readPatch(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var out patchDoc
	if err := p.Apply(existing, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "new" {
		t.Errorf("present key should replace: name=%q", out.Name)
	}
	if out.Count != 3 || !out.Enabled || out.Big != 9007199254740993 {
		t.Errorf("absent keys should be kept exactly: count=%d enabled=%v big=%d", out.Count, out.Enabled, out.Big)
	}
	if len(out.Tags) != 1 || out.Tags[0] != "only" {
		t.Errorf("arrays should replace as a whole: %v", out.Tags)
	}
	if out.Note != nil {
		t.Errorf("null should clear: note=%q", *out.Note)
	}
	if out.Nested.A != "a" || out.Nested.B != "B2" || out.Nested.N != 0 {
		t.Errorf("nested objects should merge per key (a kept, b replaced, n cleared): %+v", out.Nested)
	}
	if out.Labels["k"] != "v" || out.Labels["k2"] != "v2" {
		t.Errorf("map merge: %v", out.Labels)
	}
	if !p.Has("note") || p.Has("count") {
		t.Error("Has should report mentioned keys, null included")
	}
}

func TestReadPatchRejectsNonObjects(t *testing.T) {
	for _, body := range []string{`[]`, `null`, `"str"`, `42`, `{bad json`, ``} {
		if _, err := readPatch(strings.NewReader(body)); err == nil {
			t.Errorf("body %q should be rejected", body)
		}
	}
	if _, err := readPatch(strings.NewReader(`{}`)); err != nil {
		t.Errorf("an empty object is a valid (no-op) patch: %v", err)
	}
}
