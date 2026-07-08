package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

func (h *Handler) listBlocks(w http.ResponseWriter, r *http.Request) {
	blocks, err := h.store.ListBlocks(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, blocks)
}

func (h *Handler) createBlock(w http.ResponseWriter, r *http.Request) {
	var in model.BlockedIP
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	in.CIDR = strings.TrimSpace(in.CIDR)
	if !validIPOrCIDR(in.CIDR) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr must be a valid IP or CIDR"})
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
	in.Source = "manual"

	b, err := h.store.CreateBlock(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.reconcile(r.Context()) // regenerate deny matchers + push to Caddy
	h.audit(r, "ip.block", "ip:"+b.CIDR, b.Reason)
	writeJSON(w, http.StatusCreated, b)
}

func (h *Handler) deleteBlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteBlock(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "ip.unblock", "block:"+id, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// validIPOrCIDR accepts a bare IP or a CIDR (Caddy's remote_ip matcher takes both).
func validIPOrCIDR(s string) bool {
	if s == "" {
		return false
	}
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}
