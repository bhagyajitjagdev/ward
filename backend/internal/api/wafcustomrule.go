package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// maxSecLangLen caps a single custom rule's SecLang (sanity, not security — the
// operator is trusted; this just keeps a paste accident out of the config).
const maxSecLangLen = 64 * 1024

// customRuleInput is the request shape. Enabled is a pointer so "omitted" is
// distinguishable from an explicit false: an omitted enabled means true on
// create (a new rule should be live — and only a rendered rule gets validated
// by the edge) and keep-current on update.
type customRuleInput struct {
	Scope     string  `json:"scope"`
	ServiceID *string `json:"service_id"`
	Name      string  `json:"name"`
	SecLang   string  `json:"seclang"`
	Enabled   *bool   `json:"enabled"`
}

func (in customRuleInput) toModel(enabledDefault bool) model.WAFCustomRule {
	enabled := enabledDefault
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return model.WAFCustomRule{
		Scope: in.Scope, ServiceID: in.ServiceID, Name: in.Name,
		SecLang: in.SecLang, Enabled: enabled,
	}
}

func validateCustomRuleInput(in *model.WAFCustomRule) string {
	in.Name = strings.TrimSpace(in.Name)
	in.SecLang = strings.TrimSpace(in.SecLang)
	if in.Name == "" || in.SecLang == "" {
		return "name and seclang are required"
	}
	if len(in.SecLang) > maxSecLangLen {
		return "seclang is too large (64KB max)"
	}
	if in.Scope == "" {
		in.Scope = "global"
	}
	switch in.Scope {
	case "global":
		in.ServiceID = nil
	case "service":
		if in.ServiceID == nil || *in.ServiceID == "" {
			return "service_id is required for scope=service"
		}
	default:
		return "scope must be 'global' or 'service'"
	}
	return ""
}

// applyOrReject pushes the regenerated config to the edge. The load is atomic:
// on failure Caddy keeps serving the previous config, so the caller only has to
// undo the DB write. Returns "" on success (or with no edge configured — dev
// mode), else the edge's rejection message.
func (h *Handler) applyOrReject(ctx context.Context) string {
	if h.applier == nil {
		return ""
	}
	if err := h.applier.Apply(ctx); err != nil {
		return edgeErrorTail(err.Error())
	}
	return ""
}

// edgeErrorTail cuts Caddy's nested provision chain ("loading module 'subroute':
// … loading module 'waf': …") down to the SecLang compile error — the part the
// operator can actually act on. Falls back to the full message.
func edgeErrorTail(msg string) string {
	if i := strings.Index(msg, "failed to compile"); i >= 0 {
		// The message is JSON-embedded-in-JSON from the admin API — unescape quotes.
		return strings.ReplaceAll(strings.TrimRight(msg[i:], `\"}`), `\"`, `"`)
	}
	return msg
}

func (h *Handler) listWAFCustomRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.store.ListWAFCustomRules(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *Handler) createWAFCustomRule(w http.ResponseWriter, r *http.Request) {
	var body customRuleInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	in := body.toModel(true) // omitted enabled → live
	if msg := validateCustomRuleInput(&in); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	rule, err := h.store.CreateWAFCustomRule(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Validate-before-keep: push the config including the new rule; if the edge
	// rejects it (bad SecLang), drop the row — the edge never changed.
	if msg := h.applyOrReject(r.Context()); msg != "" {
		_, _ = h.store.DeleteWAFCustomRule(r.Context(), rule.ID)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the edge rejected this rule: " + msg})
		return
	}
	h.audit(r, "wafrule.create", "wafrule:"+rule.ID, rule.Name)
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) updateWAFCustomRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body customRuleInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	prev, found, err := h.store.GetWAFCustomRule(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	in := body.toModel(prev.Enabled) // omitted enabled → keep current
	if msg := validateCustomRuleInput(&in); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	rule, found, err := h.store.UpdateWAFCustomRule(r.Context(), id, in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// Same validate-before-keep as create: on rejection restore the previous row.
	if msg := h.applyOrReject(r.Context()); msg != "" {
		_, _, _ = h.store.UpdateWAFCustomRule(r.Context(), id, prev)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the edge rejected this rule: " + msg})
		return
	}
	h.audit(r, "wafrule.update", "wafrule:"+rule.ID, rule.Name)
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) deleteWAFCustomRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteWAFCustomRule(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "wafrule.delete", "wafrule:"+id, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
