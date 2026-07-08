package api

import (
	"encoding/json"
	"net/http"

	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

type settingsDTO struct {
	WAFEngineMode string `json:"waf_engine_mode"` // "DetectionOnly" | "On" — the global default
}

// validWAFMode reports whether m is a valid engine mode. Empty is valid only for a
// per-service override, where it means "inherit the global default".
func validWAFMode(m string) bool {
	return m == "" || m == "DetectionOnly" || m == "On"
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, settingsDTO{
		WAFEngineMode: h.store.WAFEngineMode(r.Context(), "DetectionOnly"),
	})
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.WAFEngineMode != "DetectionOnly" && in.WAFEngineMode != "On" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "waf_engine_mode must be 'DetectionOnly' or 'On'"})
		return
	}
	if err := h.store.SetSetting(r.Context(), store.WAFModeKey, in.WAFEngineMode); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Regenerate: services inheriting the global default flip to the new mode.
	h.reconcile(r.Context())
	h.audit(r, "settings.update", store.WAFModeKey, in.WAFEngineMode)
	writeJSON(w, http.StatusOK, settingsDTO{WAFEngineMode: in.WAFEngineMode})
}
