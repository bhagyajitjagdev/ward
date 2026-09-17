package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func (h *Handler) listRateLimits(w http.ResponseWriter, r *http.Request) {
	rls, err := h.store.ListRateLimits(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rls)
}

// validateRateLimit checks a fully-populated rate limit (shared by create and update).
func validateRateLimit(in *model.RateLimit) (int, string) {
	if in.MaxEvents <= 0 {
		return http.StatusBadRequest, "max_events must be greater than 0"
	}
	if _, err := time.ParseDuration(in.Window); err != nil {
		return http.StatusBadRequest, "window must be a duration like 1m, 10s, or 1h"
	}
	if in.Scope == "" {
		in.Scope = "global"
	}
	switch in.Scope {
	case "global":
		in.ServiceID = nil
	case "service":
		if in.ServiceID == nil || *in.ServiceID == "" {
			return http.StatusBadRequest, "service_id is required for scope=service"
		}
	default:
		return http.StatusBadRequest, "scope must be 'global' or 'service'"
	}
	return 0, ""
}

func (h *Handler) createRateLimit(w http.ResponseWriter, r *http.Request) {
	var in model.RateLimit
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateRateLimit(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}

	rl, err := h.store.CreateRateLimit(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "ratelimit.create", "ratelimit:"+rl.ID, rl.Scope)
	writeJSON(w, http.StatusCreated, rl)
}

// updateRateLimit is a JSON Merge Patch: only the fields in the body change.
func (h *Handler) updateRateLimit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, found, err := h.store.GetRateLimit(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	p, err := readPatch(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var in model.RateLimit
	if err := p.Apply(existing, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateRateLimit(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	rl, found, err := h.store.UpdateRateLimit(r.Context(), id, in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "ratelimit.update", "ratelimit:"+rl.ID, rl.Scope)
	writeJSON(w, http.StatusOK, rl)
}

func (h *Handler) deleteRateLimit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteRateLimit(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "ratelimit.delete", "ratelimit:"+id, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
