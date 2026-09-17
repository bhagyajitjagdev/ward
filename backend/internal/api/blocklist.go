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

// validateBlock normalizes + checks a fully-populated IP rule (shared by create and
// update). An empty scope/mode (a null in a patch) means the default: global / block.
func validateBlock(in *model.BlockedIP) (int, string) {
	in.CIDR = strings.TrimSpace(in.CIDR)
	if !validIPOrCIDR(in.CIDR) {
		return http.StatusBadRequest, "cidr must be a valid IP or CIDR"
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

func (h *Handler) createBlock(w http.ResponseWriter, r *http.Request) {
	var in model.BlockedIP
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateBlock(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	in.Source = "manual"

	b, err := h.store.CreateBlock(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.reconcile(r.Context()) // regenerate matchers + push to Caddy
	action := "ip.block"
	if b.Mode == "allow" {
		action = "ip.allow"
	}
	h.audit(r, action, "ip:"+b.CIDR, b.Reason)
	writeJSON(w, http.StatusCreated, b)
}

// updateBlock is a JSON Merge Patch: only the fields in the body change, null clears
// (e.g. `"expires_at": null` makes a temporary ban permanent), the rest is kept.
func (h *Handler) updateBlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, found, err := h.store.GetBlock(r.Context(), id)
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
	var in model.BlockedIP
	if err := p.Apply(existing, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if code, msg := validateBlock(&in); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	b, found, err := h.store.UpdateBlock(r.Context(), id, in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "ip.update", "ip:"+b.CIDR, b.Reason)
	writeJSON(w, http.StatusOK, b)
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
