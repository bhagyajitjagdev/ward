package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func (h *Handler) listGeoRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.store.ListGeoRules(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) createGeoRule(w http.ResponseWriter, r *http.Request) {
	var in model.GeoRule
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// normalize to uppercase 2-letter ISO codes, deduped
	seen := map[string]bool{}
	codes := make([]string, 0, len(in.Countries))
	for _, c := range in.Countries {
		c = strings.ToUpper(strings.TrimSpace(c))
		if len(c) == 2 && !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	in.Countries = codes
	if len(in.Countries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one 2-letter country code is required"})
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
	if in.Mode == "" {
		in.Mode = "block"
	}
	if in.Mode != "block" && in.Mode != "allow" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be 'block' or 'allow'"})
		return
	}

	rule, err := h.store.CreateGeoRule(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "geo.create", "geo:"+rule.ID, strings.Join(rule.Countries, ","))
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateGeoRule(w http.ResponseWriter, r *http.Request) {
	var in model.GeoRule
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	seen := map[string]bool{}
	codes := make([]string, 0, len(in.Countries))
	for _, c := range in.Countries {
		c = strings.ToUpper(strings.TrimSpace(c))
		if len(c) == 2 && !seen[c] {
			seen[c] = true
			codes = append(codes, c)
		}
	}
	in.Countries = codes
	if len(in.Countries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one 2-letter country code is required"})
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
	if in.Mode == "" {
		in.Mode = "block"
	}
	if in.Mode != "block" && in.Mode != "allow" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be 'block' or 'allow'"})
		return
	}
	rule, found, err := h.store.UpdateGeoRule(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "geo.update", "geo:"+rule.ID, strings.Join(rule.Countries, ","))
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) deleteGeoRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteGeoRule(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "geo.delete", "geo:"+id, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
