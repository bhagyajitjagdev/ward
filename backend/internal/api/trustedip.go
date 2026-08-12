package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func (h *Handler) listTrusted(w http.ResponseWriter, r *http.Request) {
	list, err := h.store.ListTrusted(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) createTrusted(w http.ResponseWriter, r *http.Request) {
	var in model.TrustedIP
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	in.CIDR = strings.TrimSpace(in.CIDR)
	if !validIPOrCIDR(in.CIDR) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr must be a valid IP or CIDR"})
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	t, err := h.store.CreateTrusted(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.reconcile(r.Context()) // regenerate matchers + push to Caddy
	h.audit(r, "ip.trust", "ip:"+t.CIDR, t.Note)
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) deleteTrusted(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteTrusted(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "ip.untrust", "trusted:"+id, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
