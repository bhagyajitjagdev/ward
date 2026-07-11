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

func (h *Handler) createRateLimit(w http.ResponseWriter, r *http.Request) {
	var in model.RateLimit
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.MaxEvents <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max_events must be greater than 0"})
		return
	}
	if _, err := time.ParseDuration(in.Window); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "window must be a duration like 1m, 10s, or 1h"})
		return
	}
	if in.Scope == "" {
		in.Scope = "global"
	}
	switch in.Scope {
	case "global":
		in.ServiceID = nil
	case "service":
		if in.ServiceID == nil || *in.ServiceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service_id is required for scope=service"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be 'global' or 'service'"})
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

func (h *Handler) updateRateLimit(w http.ResponseWriter, r *http.Request) {
	var in model.RateLimit
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.MaxEvents <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max_events must be greater than 0"})
		return
	}
	if _, err := time.ParseDuration(in.Window); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "window must be a duration like 1m, 10s, or 1h"})
		return
	}
	if in.Scope == "" {
		in.Scope = "global"
	}
	switch in.Scope {
	case "global":
		in.ServiceID = nil
	case "service":
		if in.ServiceID == nil || *in.ServiceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service_id is required for scope=service"})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be 'global' or 'service'"})
		return
	}
	rl, found, err := h.store.UpdateRateLimit(r.Context(), r.PathValue("id"), in)
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
