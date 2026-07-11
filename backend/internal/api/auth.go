package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/auth"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

const sessionTTL = 7 * 24 * time.Hour

// authMiddleware requires a valid bearer session for all non-public routes and
// attaches the authenticated user to the request context.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		tok := bearer(r)
		if tok == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		user, err := h.store.UserFromBearer(r.Context(), auth.HashToken(tok))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), user)))
	})
}

func isPublic(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/healthz":
		return true
	case method == http.MethodPost && path == "/auth/setup":
		return true
	case method == http.MethodPost && path == "/auth/login":
		return true
	case method == http.MethodGet && path == "/auth/state":
		return true
	}
	return false
}

func bearer(r *http.Request) string {
	if after, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// audit appends an entry attributed to the current user (or "system").
func (h *Handler) audit(r *http.Request, action, target, detail string) {
	actor := "system"
	if u := auth.UserFromContext(r.Context()); u != nil {
		actor = u.Username
	}
	_ = h.store.WriteAudit(r.Context(), actor, action, target, detail)
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if n > 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "already set up"})
		return
	}
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if len(in.Username) < 3 || len(in.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username (>=3) and password (>=8) required"})
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	user, err := h.store.CreateUser(r.Context(), in.Username, hash, "admin", true)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	tok, exp, err := h.newSession(r, user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.WriteAudit(r.Context(), user.Username, "auth.setup", "user:"+user.ID, "owner account created")
	writeJSON(w, http.StatusCreated, loginResponse(tok, exp, user))
}

// authState reports whether first-run setup is still needed (no users yet). Public
// so the UI can route between /setup and /login before anyone is authenticated.
func (h *Handler) authState(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.CountUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	user, err := h.store.AuthenticateUser(r.Context(), in.Username, in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	tok, exp, err := h.newSession(r, *user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse(tok, exp, *user))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if tok := bearer(r); tok != "" {
		_ = h.store.DeleteSession(r.Context(), auth.HashToken(tok))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, auth.UserFromContext(r.Context()))
}

func (h *Handler) newSession(r *http.Request, user model.User) (string, time.Time, error) {
	raw, hash, err := auth.GenerateToken()
	if err != nil {
		return "", time.Time{}, err
	}
	exp, err := h.store.CreateSession(r.Context(), user.ID, hash, sessionTTL)
	return raw, exp, err
}

func loginResponse(token string, exp time.Time, user model.User) map[string]any {
	return map[string]any{"token": token, "expires_at": exp, "user": user}
}

// --- user management (any admin) ---

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if len(in.Username) < 3 || len(in.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username (>=3) and password (>=8) required"})
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	user, err := h.store.CreateUser(r.Context(), in.Username, hash, "admin", false)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.audit(r, "user.create", "user:"+user.ID, user.Username)
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	target, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if target == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if target.IsOwner {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "the owner account cannot be deleted"})
		return
	}
	if _, err := h.store.DeleteUser(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.audit(r, "user.delete", "user:"+id, target.Username)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listAudit(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	entries, err := h.store.ListAudit(r.Context(), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
