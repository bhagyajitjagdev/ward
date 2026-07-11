// Package caddy generates Caddy's JSON config from Ward's DB state and drives
// the Caddy admin API (the config write-path). Ward is the only writer of
// Caddy config; the running config is always derived from the DB.
package caddy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bhagyajitjagdev/ward/backend/internal/certs"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// Options controls config generation.
type Options struct {
	AdminListen      string // Caddy admin bind, e.g. "0.0.0.0:2019" (internal network only)
	HTTPPort         string // ":80"
	HTTPSPort        string // ":443"
	DisableAutoHTTPS bool   // skip ACME/redirects (dev / no public domains)
	WAFEngineMode    string // "DetectionOnly" | "On"
	AuditLogPath     string // where Coraza writes its JSON audit log
	ACMEEmail        string // contact email for managed (Let's Encrypt) certs
	GeoIPDBPath      string // path to the GeoIP .mmdb for the geo matcher (empty → geo blocking off)
	AccessLogPath    string // where Caddy writes the JSON access log (empty → access logging off)
}

// DefaultOptions returns sane defaults.
func DefaultOptions() Options {
	return Options{
		AdminListen:   "0.0.0.0:2019",
		HTTPPort:      ":80",
		HTTPSPort:     ":443",
		WAFEngineMode: "DetectionOnly",
		AuditLogPath:  "/dev/stdout",
	}
}

// CustomCert is a bring-your-own certificate resolved from the certs volume:
// the PEM file paths Caddy loads (via load_files) for a tls_mode=custom host.
type CustomCert struct {
	Domain   string   // the cert's upload label / storage folder
	Subjects []string // CN + SANs — the hosts this cert actually secures
	CertPath string
	KeyPath  string
}

// ResolveCustomCerts loads the uploaded bring-your-own certs from the certs
// volume into load specs for Generate. A missing/unreadable volume → none.
func ResolveCustomCerts() []CustomCert {
	list, err := certs.List(certs.Dir())
	if err != nil {
		return nil
	}
	out := make([]CustomCert, 0, len(list))
	for _, c := range list {
		out = append(out, CustomCert{Domain: c.Domain, Subjects: c.Subjects, CertPath: c.CertPath, KeyPath: c.KeyPath})
	}
	return out
}

// certForHost returns the uploaded cert whose SAN covers host (wildcard-aware), or
// nil. One multi-SAN / wildcard cert therefore serves every host it lists — no need
// to re-upload it per service.
func certForHost(cc []CustomCert, host string) *CustomCert {
	for i := range cc {
		for _, s := range cc[i].Subjects {
			if certs.SANMatches(host, s) {
				return &cc[i]
			}
		}
	}
	return nil
}

// Input is the desired edge state Generate renders into a Caddy config.
type Input struct {
	Services     []model.Service
	Exclusions   []model.WAFExclusion
	Blocks       []model.BlockedIP
	RateLimits   []model.RateLimit
	GeoRules     []model.GeoRule
	Certificates []CustomCert
}

// Generate builds a full Caddy JSON config from the desired edge state. Global
// rules (IP deny, rate limit, geo deny) apply at the top of the server; per-service
// rules apply inside that service's route.
func Generate(in Input, opt Options) ([]byte, error) {
	services, exclusions, blocks, rateLimits, geoRules := in.Services, in.Exclusions, in.Blocks, in.RateLimits, in.GeoRules

	var globalExcl []string
	exclByService := map[string][]string{}
	for _, ex := range exclusions {
		if ex.State != "active" || ex.SecLang == "" {
			continue
		}
		switch {
		case ex.Scope == "global":
			globalExcl = append(globalExcl, ex.SecLang)
		case ex.ServiceID != nil:
			exclByService[*ex.ServiceID] = append(exclByService[*ex.ServiceID], ex.SecLang)
		}
	}

	var globalBlocks []model.BlockedIP
	blocksByService := map[string][]model.BlockedIP{}
	for _, b := range blocks {
		if b.CIDR == "" {
			continue
		}
		switch {
		case b.Scope == "global":
			globalBlocks = append(globalBlocks, b)
		case b.ServiceID != nil:
			blocksByService[*b.ServiceID] = append(blocksByService[*b.ServiceID], b)
		}
	}

	var globalRLs []model.RateLimit
	rlsByService := map[string][]model.RateLimit{}
	for _, rl := range rateLimits {
		if rl.MaxEvents <= 0 || rl.Window == "" {
			continue
		}
		switch {
		case rl.Scope == "global":
			globalRLs = append(globalRLs, rl)
		case rl.ServiceID != nil:
			rlsByService[*rl.ServiceID] = append(rlsByService[*rl.ServiceID], rl)
		}
	}

	var globalGeo []model.GeoRule
	geoByService := map[string][]model.GeoRule{}
	for _, g := range geoRules {
		if len(g.Countries) == 0 {
			continue
		}
		switch {
		case g.Scope == "global":
			globalGeo = append(globalGeo, g)
		case g.ServiceID != nil:
			geoByService[*g.ServiceID] = append(geoByService[*g.ServiceID], g)
		}
	}

	var internalSubs, managedSubs, skipSubs, customSubs []string
	routes := make([]any, 0, len(services)*2+3)
	for _, r := range ipRoutes(globalBlocks) {
		routes = append(routes, r) // edge-wide IP deny + allow-only gate
	}
	for _, r := range geoRoutes(globalGeo, opt.GeoIPDBPath) {
		routes = append(routes, r) // edge-wide geo block + allow-only gate
	}
	if len(globalRLs) > 0 {
		// pass-through middleware: caps every IP edge-wide, then continues to the service routes
		routes = append(routes, map[string]any{"handle": []any{rateLimitHandler(globalRLs)}})
	}
	var redirectRoutes, svcRoutes []any
	for _, s := range services {
		if !s.Enabled || len(s.Upstreams) == 0 {
			continue
		}
		excl := append(append([]string{}, globalExcl...), exclByService[s.ID]...)
		svcRoutes = append(svcRoutes, serviceRoute(s, opt, excl, blocksByService[s.ID], rlsByService[s.ID], geoByService[s.ID]))
		switch s.TLSMode {
		case "none":
			skipSubs = append(skipSubs, s.PublicHostname)
			continue // HTTP-only: no HTTPS to redirect to
		case "managed":
			managedSubs = append(managedSubs, s.PublicHostname)
		case "custom":
			customSubs = append(customSubs, s.PublicHostname)
		default: // "internal" or unset → local self-signed CA
			internalSubs = append(internalSubs, s.PublicHostname)
		}
		// Any cert-bearing service forces HTTP -> HTTPS (a cert implies HTTPS).
		redirectRoutes = append(redirectRoutes, httpsRedirectRoute(s.PublicHostname))
	}
	// Redirects fire before the service content routes; skipped in the no-auto-HTTPS
	// (dev) path, where there's no HTTPS to redirect to.
	if !opt.DisableAutoHTTPS {
		routes = append(routes, redirectRoutes...)
	}
	routes = append(routes, svcRoutes...)

	server := map[string]any{"routes": routes}
	if opt.AccessLogPath != "" {
		server["logs"] = map[string]any{} // enable access logging for this server
	}
	var tlsApp map[string]any
	if opt.DisableAutoHTTPS {
		server["listen"] = []string{opt.HTTPPort}
		server["automatic_https"] = map[string]any{"disable": true}
	} else {
		server["listen"] = []string{opt.HTTPPort, opt.HTTPSPort}
		// Ward emits explicit per-host HTTP->HTTPS redirects (above), so turn off
		// Caddy's automatic global one.
		autoHTTPS := map[string]any{"disable_redirects": true}
		if len(skipSubs) > 0 {
			autoHTTPS["skip"] = skipSubs // HTTP-only services: no TLS at all
		}
		// Bring-your-own-cert hosts: stop Caddy auto-issuing for them (skip_certificates
		// keeps the HTTP->HTTPS redirect), and load the uploaded PEM files from the volume.
		var loadFiles []any
		if len(customSubs) > 0 {
			loadedPaths := map[string]bool{} // a multi-SAN cert covers many hosts — load it once
			var served []string
			for _, host := range customSubs {
				c := certForHost(in.Certificates, host)
				if c == nil {
					continue
				}
				if !loadedPaths[c.CertPath] {
					loadFiles = append(loadFiles, map[string]any{"certificate": c.CertPath, "key": c.KeyPath})
					loadedPaths[c.CertPath] = true
				}
				served = append(served, host)
			}
			if len(served) > 0 {
				autoHTTPS["skip_certificates"] = served
			}
		}
		if len(autoHTTPS) > 0 {
			server["automatic_https"] = autoHTTPS
		}
		var policies []any
		if len(internalSubs) > 0 {
			policies = append(policies, map[string]any{
				"subjects": internalSubs,
				"issuers":  []any{map[string]any{"module": "internal"}},
			})
		}
		if len(managedSubs) > 0 {
			issuer := map[string]any{"module": "acme"}
			if opt.ACMEEmail != "" {
				issuer["email"] = opt.ACMEEmail
			}
			policies = append(policies, map[string]any{
				"subjects": managedSubs,
				"issuers":  []any{issuer},
			})
		}
		if len(policies) > 0 || len(loadFiles) > 0 {
			tlsApp = map[string]any{}
			if len(policies) > 0 {
				tlsApp["automation"] = map[string]any{"policies": policies}
			}
			if len(loadFiles) > 0 {
				tlsApp["certificates"] = map[string]any{"load_files": loadFiles}
			}
		}
	}

	apps := map[string]any{
		"http": map[string]any{"servers": map[string]any{"edge": server}},
	}
	if tlsApp != nil {
		apps["tls"] = tlsApp
	}
	cfg := map[string]any{
		"admin": map[string]any{"listen": opt.AdminListen},
		"apps":  apps,
	}
	if opt.AccessLogPath != "" {
		// Route access logs to a JSON file (tailed by Ward + any external pipeline);
		// keep them out of the default (stderr) log.
		cfg["logging"] = map[string]any{"logs": map[string]any{
			"default": map[string]any{"exclude": []string{"http.log.access"}},
			"access": map[string]any{
				"writer":  map[string]any{"output": "file", "filename": opt.AccessLogPath},
				"encoder": map[string]any{"format": "json"},
				"include": []string{"http.log.access"},
			},
		}}
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// httpsRedirectRoute returns an HTTP-only 308 redirect to the HTTPS URL for host.
// The scheme match lets HTTPS requests fall through to the service's own route.
func httpsRedirectRoute(host string) map[string]any {
	return map[string]any{
		"match": []any{map[string]any{
			"host":       []string{host},
			"expression": "{http.request.scheme} == 'http'",
		}},
		"handle": []any{map[string]any{
			"handler": "static_response",
			// 302 (temporary), not 308 (permanent): browsers cache a permanent redirect
			// hard, so flipping a service to tls_mode=none would leave clients stuck
			// forcing HTTPS with no cert (ERR_SSL_PROTOCOL_ERROR). 302 recovers cleanly.
			"status_code": "302",
			"headers":     map[string]any{"Location": []string{"https://{http.request.host}{http.request.uri}"}},
		}},
	}
}

// denyRoute returns 403 for any request from the given IPs/CIDRs.
func denyRoute(cidrs []string) map[string]any {
	return map[string]any{
		"match": []any{map[string]any{"remote_ip": map[string]any{"ranges": cidrs}}},
		"handle": []any{map[string]any{
			"handler":     "static_response",
			"status_code": "403",
			"body":        "blocked",
		}},
	}
}

// ipRoutes turns a scope's IP rules into up to two routes: a deny route for
// block-mode entries, plus an allow-only gate (403 anything not listed) when any
// allow-mode entries exist. Allow entries are unioned so multiple don't cancel out.
func ipRoutes(blocks []model.BlockedIP) []map[string]any {
	var deny, allow []string
	for _, b := range blocks {
		if b.Mode == "allow" {
			allow = append(allow, b.CIDR)
		} else {
			deny = append(deny, b.CIDR)
		}
	}
	var out []map[string]any
	if len(deny) > 0 {
		out = append(out, denyRoute(deny))
	}
	if len(allow) > 0 {
		out = append(out, allowOnlyIPRoute(allow))
	}
	return out
}

// allowOnlyIPRoute returns 403 for any client NOT in the given IPs/CIDRs.
func allowOnlyIPRoute(cidrs []string) map[string]any {
	return map[string]any{
		"match": []any{map[string]any{
			"not": []any{map[string]any{"remote_ip": map[string]any{"ranges": cidrs}}},
		}},
		"handle": []any{map[string]any{
			"handler":     "static_response",
			"status_code": "403",
			"body":        "blocked",
		}},
	}
}

// geoRoutes turns a scope's geo rules into up to two routes (block-mode deny +
// allow-only gate). No-op when no GeoIP database is configured.
func geoRoutes(rules []model.GeoRule, dbPath string) []map[string]any {
	if dbPath == "" {
		return nil
	}
	var deny, allow []string
	for _, g := range rules {
		if len(g.Countries) == 0 {
			continue
		}
		if g.Mode == "allow" {
			allow = append(allow, g.Countries...)
		} else {
			deny = append(deny, g.Countries...)
		}
	}
	var out []map[string]any
	if len(deny) > 0 {
		out = append(out, geoDenyRoute(deny, dbPath))
	}
	if len(allow) > 0 {
		out = append(out, geoAllowRoute(allow, dbPath))
	}
	return out
}

// geoAllowRoute returns 403 for any request NOT from the given countries.
func geoAllowRoute(countries []string, dbPath string) map[string]any {
	return map[string]any{
		"match": []any{map[string]any{
			"not": []any{map[string]any{
				"maxmind_geolocation": map[string]any{"db_path": dbPath, "allow_countries": countries},
			}},
		}},
		"handle": []any{map[string]any{
			"handler":     "static_response",
			"status_code": "403",
			"body":        "blocked (geo)",
		}},
	}
}

// rateLimitHandler builds a caddy-ratelimit handler with one zone per rate limit,
// each keyed by client IP. Over the cap → 429.
func rateLimitHandler(rls []model.RateLimit) map[string]any {
	zones := map[string]any{}
	for _, rl := range rls {
		zones[rl.ID] = map[string]any{
			"key":        "{http.request.remote_host}",
			"window":     rl.Window,
			"max_events": rl.MaxEvents,
		}
	}
	return map[string]any{
		"handler":     "rate_limit",
		"rate_limits": zones,
	}
}

// geoDenyRoute returns 403 for requests from the given countries.
//
// NOTE: the porech maxmind_geolocation matcher *matches* a request whose country
// is in allow_countries, and does NOT match one in deny_countries. So to BLOCK a
// set of countries with a 403-on-match route we list them under allow_countries —
// the matcher then fires precisely for those countries. Using deny_countries here
// inverts the logic (it would 403 everyone *except* the listed countries), which
// is the mirror of geoAllowRoute's `not { allow_countries }`.
func geoDenyRoute(countries []string, dbPath string) map[string]any {
	return map[string]any{
		"match": []any{map[string]any{
			"maxmind_geolocation": map[string]any{
				"db_path":         dbPath,
				"allow_countries": countries,
			},
		}},
		"handle": []any{map[string]any{
			"handler":     "static_response",
			"status_code": "403",
			"body":        "blocked (geo)",
		}},
	}
}

// GenerateExclusionSecLang builds the SecLang for one scoped exclusion. With a
// path it's a runtime rule (the wedge form: silence a rule's target on a path);
// without a path it's a configure-time rule affecting the whole handler.
func GenerateExclusionSecLang(seclangID, ruleID int, path, target string) string {
	var ctl string
	if target != "" {
		ctl = fmt.Sprintf("ctl:ruleRemoveTargetById=%d;%s", ruleID, target)
	} else {
		ctl = fmt.Sprintf("ctl:ruleRemoveById=%d", ruleID)
	}
	if path != "" {
		return fmt.Sprintf(`SecRule REQUEST_URI "@beginsWith %s" "id:%d,phase:1,pass,nolog,%s"`, path, seclangID, ctl)
	}
	if target != "" {
		return fmt.Sprintf(`SecRuleUpdateTargetById %d "!%s"`, ruleID, target)
	}
	return fmt.Sprintf("SecRuleRemoveById %d", ruleID)
}

// serviceRoute builds one host-matched route: [ip rules] → [geo rules] → [rate_limit?] → [waf?] → reverse_proxy.
func serviceRoute(s model.Service, opt Options, exclusions []string, blocks []model.BlockedIP, rateLimits []model.RateLimit, geoRules []model.GeoRule) map[string]any {
	innerRoutes := make([]any, 0, 4)
	for _, r := range ipRoutes(blocks) {
		innerRoutes = append(innerRoutes, r)
	}
	for _, r := range geoRoutes(geoRules, opt.GeoIPDBPath) {
		innerRoutes = append(innerRoutes, r)
	}

	handlers := make([]any, 0, 3)
	if len(rateLimits) > 0 {
		handlers = append(handlers, rateLimitHandler(rateLimits)) // cheap 429 before WAF work
	}
	if s.WAFEnabled {
		mode := s.WAFMode
		if mode == "" {
			mode = opt.WAFEngineMode // inherit the global default
		}
		handlers = append(handlers, wafHandler(opt, mode, exclusions))
	}
	handlers = append(handlers, reverseProxyHandler(s))
	innerRoutes = append(innerRoutes, map[string]any{"handle": handlers})

	return map[string]any{
		"match": []any{map[string]any{"host": []string{s.PublicHostname}}},
		"handle": []any{
			map[string]any{"handler": "subroute", "routes": innerRoutes},
		},
	}
}

func reverseProxyHandler(s model.Service) map[string]any {
	ups := make([]any, 0, len(s.Upstreams))
	for _, u := range s.Upstreams {
		ups = append(ups, map[string]any{"dial": u})
	}
	h := map[string]any{"handler": "reverse_proxy", "upstreams": ups}
	if s.LBPolicy != "" && s.LBPolicy != "round_robin" {
		h["load_balancing"] = map[string]any{
			"selection_policy": map[string]any{"policy": s.LBPolicy},
		}
	}
	if len(ups) > 1 {
		// Passive health checks: pull a replica out of rotation after repeated
		// failures so load-balanced traffic skips a dead/erroring upstream.
		h["health_checks"] = map[string]any{
			"passive": map[string]any{
				"fail_duration":    "30s",
				"max_fails":        3,
				"unhealthy_status": []any{500, 502, 503, 504},
			},
		}
	}
	return h
}

func wafHandler(opt Options, mode string, exclusions []string) map[string]any {
	return map[string]any{
		"handler":        "waf",
		"load_owasp_crs": true,
		"directives":     buildDirectives(opt, mode, exclusions),
	}
}

// buildDirectives is the per-service SecLang: CRS setup → Ward-managed exclusions
// → the CRS rules → engine mode → audit logging. mode is the service's effective
// engine mode (its override, else the global default).
//
// Exclusions are emitted BEFORE the CRS rules include (the CRS "before-CRS" slot):
// a ctl:ruleRemoveById only suppresses a rule that has not run yet, and Coraza
// evaluates rules in directive order within a phase. Appending them after the
// include (as before) made every scoped exclusion a no-op.
func buildDirectives(opt Options, mode string, exclusions []string) string {
	if mode != "On" && mode != "DetectionOnly" {
		mode = "DetectionOnly"
	}
	audit := opt.AuditLogPath
	if audit == "" {
		audit = "/dev/stdout"
	}
	lines := []string{
		"Include @coraza.conf-recommended",
		"Include @crs-setup.conf.example",
	}
	lines = append(lines, exclusions...) // before the CRS rules, so ruleRemoveById wins
	lines = append(lines,
		"Include @owasp_crs/*.conf",
		"SecRuleEngine "+mode,
		"SecAuditEngine On",
		"SecAuditLog "+audit,
		"SecAuditLogType Serial",
		"SecAuditLogFormat JSON",
		"SecAuditLogParts ABIJDEFHZ",
	)
	return strings.Join(lines, "\n")
}
