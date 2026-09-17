package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// jsonPatch is a parsed JSON Merge Patch body (RFC 7396). Applied onto the existing
// resource: present keys replace, nested objects merge key by key, arrays replace as
// a whole, null clears a key, and anything absent keeps its current value. This is
// what makes every PATCH a true partial update — a client that sends one field can't
// reset the rest by omission (before this, an omitted waf_enabled turned the WAF off,
// an omitted mode flipped an allow-only IP rule to block, an omitted scope made a
// per-service rule edge-wide — all with a 200).
type jsonPatch struct{ fields map[string]any }

var errPatchNotObject = errors.New("invalid JSON body: a PATCH body must be a JSON object")

// maxPatchBytes bounds a PATCH body (raw SecLang / Caddyfile fragments are far smaller).
const maxPatchBytes = 1 << 20

// readPatch parses a request body as a merge patch. Numbers are kept verbatim
// (json.Number) so they round-trip through the merge exactly.
func readPatch(r io.Reader) (jsonPatch, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxPatchBytes))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return jsonPatch{}, errPatchNotObject
	}
	m, ok := v.(map[string]any)
	if !ok {
		return jsonPatch{}, errPatchNotObject
	}
	return jsonPatch{fields: m}, nil
}

// Has reports whether the body mentions a top-level key (a null counts as mentioned).
func (p jsonPatch) Has(key string) bool {
	_, ok := p.fields[key]
	return ok
}

// Apply merges the patch onto existing (any JSON-marshalable value) and decodes the
// result into out, which should point at a zero value of the same type.
func (p jsonPatch) Apply(existing, out any) error {
	base, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(base))
	dec.UseNumber()
	var target map[string]any
	if err := dec.Decode(&target); err != nil {
		return err
	}
	merged, err := json.Marshal(mergeJSON(target, p.fields))
	if err != nil {
		return err
	}
	return json.Unmarshal(merged, out)
}

// mergeJSON applies patch onto target per RFC 7396 and returns target.
func mergeJSON(target, patch map[string]any) map[string]any {
	if target == nil {
		target = map[string]any{}
	}
	for k, v := range patch {
		if v == nil {
			delete(target, k)
			continue
		}
		if pm, ok := v.(map[string]any); ok {
			tm, _ := target[k].(map[string]any) // a non-object (or missing) target is replaced
			target[k] = mergeJSON(tm, pm)
			continue
		}
		target[k] = v
	}
	return target
}
