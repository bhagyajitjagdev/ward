package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"sync"

	"gopkg.in/yaml.v3"
)

// The OpenAPI spec is authored in YAML next to the handlers (openapi.yaml) and
// served as JSON at GET /openapi.json for tooling (and the ward-ui codegen).
//
//go:embed openapi.yaml
var openapiYAML []byte

var specJSON = sync.OnceValues(func() ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		return nil, err
	}
	return json.Marshal(doc)
})

func (h *Handler) openapiSpec(w http.ResponseWriter, r *http.Request) {
	b, err := specJSON()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
