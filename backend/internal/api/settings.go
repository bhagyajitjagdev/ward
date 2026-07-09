package api

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strconv"

	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

type settingsDTO struct {
	WAFEngineMode       string `json:"waf_engine_mode"`       // "DetectionOnly" | "On" — the global default
	ACMEEmail           string `json:"acme_email"`            // contact email for managed (Let's Encrypt) certs
	AccessRetentionDays int    `json:"access_retention_days"` // days of raw access events to keep
}

// validWAFMode reports whether m is a valid engine mode. Empty is valid only for a
// per-service override, where it means "inherit the global default".
func validWAFMode(m string) bool {
	return m == "" || m == "DetectionOnly" || m == "On"
}

func (h *Handler) currentSettings(r *http.Request) settingsDTO {
	return settingsDTO{
		WAFEngineMode:       h.store.WAFEngineMode(r.Context(), "DetectionOnly"),
		ACMEEmail:           h.store.ACMEEmail(r.Context(), ""),
		AccessRetentionDays: h.store.AccessRetentionDays(r.Context(), 7),
	}
}

func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.currentSettings(r))
}

// updateSettings applies whichever fields are present (non-empty); each is
// validated and audited independently, then the config is reconciled once.
func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	changed := false
	if in.WAFEngineMode != "" {
		if in.WAFEngineMode != "DetectionOnly" && in.WAFEngineMode != "On" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "waf_engine_mode must be 'DetectionOnly' or 'On'"})
			return
		}
		if err := h.store.SetSetting(r.Context(), store.WAFModeKey, in.WAFEngineMode); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		h.audit(r, "settings.update", store.WAFModeKey, in.WAFEngineMode)
		changed = true
	}
	if in.ACMEEmail != "" {
		if _, err := mail.ParseAddress(in.ACMEEmail); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "acme_email must be a valid email address"})
			return
		}
		if err := h.store.SetSetting(r.Context(), store.ACMEEmailKey, in.ACMEEmail); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		h.audit(r, "settings.update", store.ACMEEmailKey, in.ACMEEmail)
		changed = true
	}
	if in.AccessRetentionDays > 0 {
		if in.AccessRetentionDays > 365 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "access_retention_days must be between 1 and 365"})
			return
		}
		if err := h.store.SetSetting(r.Context(), store.AccessRetentionKey, strconv.Itoa(in.AccessRetentionDays)); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		h.audit(r, "settings.update", store.AccessRetentionKey, strconv.Itoa(in.AccessRetentionDays))
		changed = true
	}
	if !changed {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nothing to update"})
		return
	}
	// Regenerate: services inheriting the global WAF default flip; the ACME issuer picks up the email.
	h.reconcile(r.Context())
	writeJSON(w, http.StatusOK, h.currentSettings(r))
}
