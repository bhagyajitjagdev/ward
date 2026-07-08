package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/auth"
)

func (h *Handler) listAPITokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListAPITokens(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) createAPIToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}

	rand, _, err := auth.GenerateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	raw := "wt_" + rand
	tok, err := h.store.CreateAPIToken(r.Context(), in.Name, user.ID, auth.HashToken(raw), in.ExpiresAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	tok.Token = raw // shown once, never stored
	h.audit(r, "apitoken.create", "apitoken:"+tok.ID, tok.Name)
	writeJSON(w, http.StatusCreated, tok)
}

func (h *Handler) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.RevokeAPIToken(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "apitoken.revoke", "apitoken:"+id, "")
	w.WriteHeader(http.StatusNoContent)
}
