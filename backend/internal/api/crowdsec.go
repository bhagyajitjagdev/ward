package api

import (
	"net/http"

	"github.com/bhagyajitjagdev/ward/backend/internal/crowdsec"
)

type crowdsecStatusDTO struct {
	Configured bool                `json:"configured"` // LAPI URL + key present (env)
	Enabled    bool                `json:"enabled"`    // bouncer wired into the edge (toggle)
	Reachable  bool                `json:"reachable"`  // LAPI answered
	Error      string              `json:"error,omitempty"`
	Decisions  []crowdsec.Decision `json:"decisions"`
}

// crowdsecStatus reports whether CrowdSec is configured/enabled and, if reachable,
// the active decisions (read-only — the bouncer enforces them at the edge).
func (h *Handler) crowdsecStatus(w http.ResponseWriter, r *http.Request) {
	out := crowdsecStatusDTO{
		Configured: h.crowdsec != nil,
		Enabled:    h.store.CrowdSecEnabled(r.Context(), h.crowdsec != nil),
		Decisions:  []crowdsec.Decision{},
	}
	if h.crowdsec == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	decisions, err := h.crowdsec.Decisions(r.Context())
	if err != nil {
		out.Error = err.Error()
		writeJSON(w, http.StatusOK, out) // reachable=false; not a server error, just LAPI down
		return
	}
	out.Reachable = true
	if decisions != nil {
		out.Decisions = decisions
	}
	writeJSON(w, http.StatusOK, out)
}
