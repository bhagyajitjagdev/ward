// Package api exposes the Ward control-plane HTTP API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/auth"
	"github.com/bhagyajitjagdev/ward/backend/internal/caddy"
	"github.com/bhagyajitjagdev/ward/backend/internal/certs"
	"github.com/bhagyajitjagdev/ward/backend/internal/crowdsec"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// normalizePathMatch validates a path match-type, defaulting empty to "prefix".
func normalizePathMatch(m string) (string, string) {
	switch m {
	case "", "prefix":
		return "prefix", ""
	case "exact", "regex":
		return m, ""
	default:
		return "", "path_match must be 'exact', 'prefix' or 'regex'"
	}
}

var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "TRACE": true, "CONNECT": true,
}

// normalizeMethods upper-cases, validates, de-dupes and sorts an HTTP-method
// filter (deterministic SecLang). Returns (methods, "") or (nil, errorMessage).
func normalizeMethods(in []string) ([]string, string) {
	seen := map[string]bool{}
	var out []string
	for _, m := range in {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if !validHTTPMethods[m] {
			return nil, "unknown HTTP method: " + m
		}
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out, ""
}

// Version is the Ward build version, set from main (ldflags-injected). Surfaced at
// GET /version for the UI's sidebar version badge. "dev" for local builds.
var Version = "dev"

// Handler wires HTTP routes to the store and the Caddy applier.
type Handler struct {
	store    *store.Store
	applier  *caddy.Applier
	crowdsec *crowdsec.Client // read-only LAPI client; nil when CrowdSec isn't configured
}

// New builds a Handler. applier may be nil (API-only mode, no edge).
func New(s *store.Store, applier *caddy.Applier) *Handler {
	return &Handler{store: s, applier: applier}
}

// WithCrowdSec attaches a read-only CrowdSec LAPI client (chainable). Pass nil to
// leave CrowdSec unconfigured — the /crowdsec endpoint then reports it as such.
func (h *Handler) WithCrowdSec(c *crowdsec.Client) *Handler {
	h.crowdsec = c
	return h
}

// Routes returns the configured router (Go 1.22+ method+path patterns).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /version", h.version)
	mux.HandleFunc("GET /openapi.json", h.openapiSpec)

	// auth + accounts
	mux.HandleFunc("GET /auth/state", h.authState)
	mux.HandleFunc("POST /auth/setup", h.setup)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/logout", h.logout)
	mux.HandleFunc("GET /auth/me", h.me)
	mux.HandleFunc("GET /users", h.listUsers)
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("DELETE /users/{id}", h.deleteUser)
	mux.HandleFunc("GET /api-tokens", h.listAPITokens)
	mux.HandleFunc("POST /api-tokens", h.createAPIToken)
	mux.HandleFunc("DELETE /api-tokens/{id}", h.revokeAPIToken)
	mux.HandleFunc("GET /audit-log", h.listAudit)

	// edge management
	mux.HandleFunc("GET /overview", h.overview)
	mux.HandleFunc("GET /services", h.listServices)
	mux.HandleFunc("POST /services", h.createService)
	mux.HandleFunc("GET /services/{id}", h.getService)
	mux.HandleFunc("PATCH /services/{id}", h.updateService)
	mux.HandleFunc("DELETE /services/{id}", h.deleteService)
	mux.HandleFunc("GET /waf-events", h.listWAFEvents)
	mux.HandleFunc("GET /waf-events/top", h.topTriggers)
	mux.HandleFunc("GET /access-events", h.listAccessEvents)
	mux.HandleFunc("GET /access-events/stats", h.accessStats)
	mux.HandleFunc("GET /waf-exclusions", h.listExclusions)
	mux.HandleFunc("POST /waf-exclusions", h.createExclusion)
	mux.HandleFunc("DELETE /waf-exclusions/{id}", h.deleteExclusion)
	mux.HandleFunc("GET /waf-custom-rules", h.listWAFCustomRules)
	mux.HandleFunc("POST /waf-custom-rules", h.createWAFCustomRule)
	mux.HandleFunc("PATCH /waf-custom-rules/{id}", h.updateWAFCustomRule)
	mux.HandleFunc("DELETE /waf-custom-rules/{id}", h.deleteWAFCustomRule)
	mux.HandleFunc("GET /blocklist", h.listBlocks)
	mux.HandleFunc("POST /blocklist", h.createBlock)
	mux.HandleFunc("PATCH /blocklist/{id}", h.updateBlock)
	mux.HandleFunc("DELETE /blocklist/{id}", h.deleteBlock)
	mux.HandleFunc("GET /rate-limits", h.listRateLimits)
	mux.HandleFunc("POST /rate-limits", h.createRateLimit)
	mux.HandleFunc("PATCH /rate-limits/{id}", h.updateRateLimit)
	mux.HandleFunc("DELETE /rate-limits/{id}", h.deleteRateLimit)
	mux.HandleFunc("GET /geo-rules", h.listGeoRules)
	mux.HandleFunc("POST /geo-rules", h.createGeoRule)
	mux.HandleFunc("PATCH /geo-rules/{id}", h.updateGeoRule)
	mux.HandleFunc("DELETE /geo-rules/{id}", h.deleteGeoRule)
	mux.HandleFunc("GET /geoip", h.geoipGet)
	mux.HandleFunc("POST /geoip/dbip", h.geoipDBIP)
	mux.HandleFunc("POST /geoip/maxmind", h.geoipMaxMind)
	mux.HandleFunc("POST /geoip/upload", h.geoipUpload)
	mux.HandleFunc("DELETE /geoip", h.geoipDelete)
	mux.HandleFunc("GET /config-snapshots", h.listSnapshots)
	mux.HandleFunc("POST /config-snapshots/{id}/rollback", h.rollback)
	mux.HandleFunc("GET /settings", h.getSettings)
	mux.HandleFunc("PATCH /settings", h.updateSettings)
	mux.HandleFunc("GET /crowdsec", h.crowdsecStatus)
	mux.HandleFunc("GET /certificates", h.listCertificates)
	mux.HandleFunc("POST /certificates", h.uploadCertificate)
	mux.HandleFunc("DELETE /certificates/{domain}", h.deleteCertificate)

	return h.authMiddleware(mux)
}

func (h *Handler) listWAFEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.WAFEventFilter{
		ServiceID: q.Get("service_id"),
		Path:      q.Get("path"),
		ClientIP:  q.Get("client_ip"),
	}
	if v := q.Get("rule_id"); v != "" {
		f.RuleID, _ = strconv.Atoi(v)
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	events, err := h.store.ListWAFEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) topTriggers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.TriggerFilter{ServiceID: q.Get("service_id")}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	triggers, err := h.store.TopTriggers(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, triggers)
}

// version reports the running Ward build. The UI compares it against GitHub's
// latest release (client-side) to show an "update available" hint.
func (h *Handler) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": Version})
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.DB.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	svcs, err := h.store.ListServices(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	for i := range svcs {
		svcs[i] = sanitizeService(svcs[i])
	}
	writeJSON(w, http.StatusOK, svcs)
}

// checkServiceHostnames normalizes in.PublicHostnames (trim/lowercase/dedupe, with a
// fallback to the single PublicHostname alias), sets the primary alias, enforces
// cross-service uniqueness (excluding excludeID), and — for custom TLS — that an
// uploaded cert covers every name. Returns (code, msg); code 0 means OK.
func (h *Handler) checkServiceHostnames(ctx context.Context, in *model.Service, excludeID string) (int, string) {
	names := in.PublicHostnames
	if len(names) == 0 && in.PublicHostname != "" {
		names = []string{in.PublicHostname}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	if len(out) == 0 {
		return http.StatusBadRequest, "at least one public hostname is required"
	}
	in.PublicHostnames = out
	in.PublicHostname = out[0]

	dup, err := h.store.HostnamesInUse(ctx, out, excludeID)
	if err != nil {
		return http.StatusInternalServerError, err.Error()
	}
	if len(dup) > 0 {
		return http.StatusConflict, "hostname already used by another service: " + strings.Join(dup, ", ")
	}
	if in.TLSMode == "custom" {
		for _, hn := range out {
			if !certs.Covers(certs.Dir(), hn) {
				return http.StatusBadRequest, "no uploaded certificate covers " + hn + " — upload one on the Certificates screen first"
			}
		}
	}
	return 0, ""
}

// prepareServiceHTTP handles the write-only basic-auth password: bcrypt it into the
// hash, keep the existing hash on update when no new password is given, or clear it
// when auth is turned off. existingHash is "" for a create.
func prepareServiceHTTP(in *model.Service, existingHash string) (int, string) {
	hc := &in.HTTP
	if hc.BasicAuthUser == "" {
		hc.BasicAuthHash = ""
		hc.BasicAuthPassword = ""
		return 0, ""
	}
	switch {
	case hc.BasicAuthPassword != "":
		hash, err := auth.HashPassword(hc.BasicAuthPassword)
		if err != nil {
			return http.StatusInternalServerError, err.Error()
		}
		hc.BasicAuthHash = hash
	case existingHash != "":
		hc.BasicAuthHash = existingHash // keep the current password
	default:
		return http.StatusBadRequest, "basic auth: a password is required"
	}
	hc.BasicAuthPassword = ""
	return 0, ""
}

// sanitizeService blanks the basic-auth secret before a service leaves the API.
func sanitizeService(s model.Service) model.Service {
	s.HTTP.BasicAuthHash = ""
	s.HTTP.BasicAuthPassword = ""
	return s
}

// normalizeSkipPaths trims and de-dupes a service's WAF skip paths and gives each a
// leading slash. Empty entries are dropped; an empty result becomes nil.
func normalizeSkipPaths(in *model.Service) {
	if len(in.WAFSkipPaths) == 0 {
		return
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in.WAFSkipPaths))
	for _, p := range in.WAFSkipPaths {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	in.WAFSkipPaths = out
}

func (h *Handler) createService(w http.ResponseWriter, r *http.Request) {
	var in model.Service
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.Name == "" || len(in.Upstreams) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and at least one upstream are required"})
		return
	}
	if !validWAFMode(in.WAFMode) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "waf_mode must be empty, 'DetectionOnly' or 'On'"})
		return
	}
	if !validTLSMode(in.TLSMode) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tls_mode must be empty, 'internal', 'managed', 'none' or 'custom'"})
		return
	}
	normalizeSkipPaths(&in)
	if code, msg := h.checkServiceHostnames(r.Context(), &in, ""); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	if code, msg := prepareServiceHTTP(&in, ""); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	if in.RawCaddy != "" {
		if _, err := caddy.AdaptFragment(in.RawCaddy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "advanced Caddyfile: " + err.Error()})
			return
		}
	}

	svc, err := h.store.CreateService(r.Context(), in)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "a service with that public_hostname already exists"})
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// DB is the source of truth; push the derived config to the edge. If the edge
	// is unreachable the service is still saved — reconcile will retry.
	h.reconcile(r.Context())
	h.audit(r, "service.create", "service:"+svc.ID, svc.PublicHostname)

	writeJSON(w, http.StatusCreated, sanitizeService(svc))
}

func (h *Handler) getService(w http.ResponseWriter, r *http.Request) {
	svc, err := h.store.GetService(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sanitizeService(svc))
}

func (h *Handler) updateService(w http.ResponseWriter, r *http.Request) {
	var in model.Service
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.Name == "" || len(in.Upstreams) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and at least one upstream are required"})
		return
	}
	if !validWAFMode(in.WAFMode) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "waf_mode must be empty, 'DetectionOnly' or 'On'"})
		return
	}
	if !validTLSMode(in.TLSMode) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tls_mode must be empty, 'internal', 'managed', 'none' or 'custom'"})
		return
	}
	normalizeSkipPaths(&in)
	if code, msg := h.checkServiceHostnames(r.Context(), &in, r.PathValue("id")); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	// Fetch the current service for the existing basic-auth hash (kept when no new
	// password is supplied). ErrNotFound surfaces below via UpdateService.
	existing, existErr := h.store.GetService(r.Context(), r.PathValue("id"))
	existingHash := ""
	if existErr == nil {
		existingHash = existing.HTTP.BasicAuthHash
	}
	if code, msg := prepareServiceHTTP(&in, existingHash); code != 0 {
		writeJSON(w, code, map[string]string{"error": msg})
		return
	}
	if in.RawCaddy != "" {
		if _, err := caddy.AdaptFragment(in.RawCaddy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "advanced Caddyfile: " + err.Error()})
			return
		}
	}

	svc, err := h.store.UpdateService(r.Context(), r.PathValue("id"), in)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if errors.Is(err, store.ErrConflict) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a service with that public_hostname already exists"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	h.reconcile(r.Context())
	h.audit(r, "service.update", "service:"+svc.ID, svc.PublicHostname)
	writeJSON(w, http.StatusOK, sanitizeService(svc))
}

func (h *Handler) deleteService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	found, err := h.store.DeleteService(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.reconcile(r.Context())
	h.audit(r, "service.delete", "service:"+id, "")
	w.WriteHeader(http.StatusNoContent)
}

// reconcile regenerates + loads the Caddy config from current DB state.
func (h *Handler) reconcile(ctx context.Context) {
	if h.applier == nil {
		return
	}
	if err := h.applier.Apply(ctx); err != nil {
		log.Printf("reconcile: failed to push config to Caddy (will retry): %v", err)
	}
}

func (h *Handler) listExclusions(w http.ResponseWriter, r *http.Request) {
	xs, err := h.store.ListExclusions(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func (h *Handler) createExclusion(w http.ResponseWriter, r *http.Request) {
	var in model.WAFExclusion
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.RuleID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rule_id is required"})
		return
	}
	if in.Scope == "" {
		in.Scope = "service"
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

	// Structured match options: path match-type + optional method filter.
	pathMatch, msg := normalizePathMatch(in.PathMatch)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	in.PathMatch = pathMatch
	methods, msg := normalizeMethods(in.Methods)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	in.Methods = methods
	// A regex path is user input — fail fast with a clear message before the edge sees it.
	if pathMatch == "regex" && in.Path != "" {
		if _, err := regexp.Compile(in.Path); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path regex: " + err.Error()})
			return
		}
	}

	// Assign a reserved SecLang rule id and generate the scoped exclusion.
	seqID, err := h.store.NextSeq(r.Context(), "exclusion_rule_id", 90000001)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	in.SecLang = caddy.GenerateExclusionSecLang(int(seqID), in.RuleID, in.PathMatch, in.Path, in.Methods, in.Target)
	in.State = "active"
	in.Source = "manual"

	ex, err := h.store.CreateExclusion(r.Context(), in)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// Push the recomposed directives. A config rejection (e.g. a regex Coraza won't
	// accept) rolls the row back; an edge-down keeps it — Ward-generated SecLang is
	// trusted and reconcile will retry.
	if rejected, emsg := h.applyEdge(r.Context()); rejected {
		_, _ = h.store.DeleteExclusion(r.Context(), ex.ID)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the edge rejected this exclusion: " + emsg})
		return
	}
	h.audit(r, "exclusion.create", "exclusion:"+ex.ID, ex.SecLang)
	writeJSON(w, http.StatusCreated, ex)
}

func (h *Handler) deleteExclusion(w http.ResponseWriter, r *http.Request) {
	found, err := h.store.DeleteExclusion(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	h.audit(r, "exclusion.delete", "exclusion:"+r.PathValue("id"), "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
