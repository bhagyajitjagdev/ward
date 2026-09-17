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

// validateGeoRule normalizes (uppercase 2-letter ISO codes, de-duped) and checks a
// fully-populated geo rule (shared by create and update).
func validateGeoRule(in *model.GeoRule) (int, string) {
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
		return http.StatusBadRequest, "at least one 2-letter country code is required"
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
	if in.Mode == "" {
		in.Mode = "block"
	}
	if in.Mode != "block" && in.Mode != "allow" {
		return http.StatusBadRequest, "mode must be 'block' or 'allow'"
	}
	return 0, ""
}

func (h *Handler) createGeoRule(w http.ResponseWriter, r *http.Request) {
	var in model.GeoRule
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateGeoRule(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
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

// updateGeoRule is a JSON Merge Patch: only the fields in the body change. Before,
// an omitted mode turned an allow-only rule into a block rule — inverting the policy.
func (h *Handler) updateGeoRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, found, err := h.store.GetGeoRule(r.Context(), id)
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
	var in model.GeoRule
	if err := p.Apply(existing, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateGeoRule(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	rule, found, err := h.store.UpdateGeoRule(r.Context(), id, in)
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
