// Package caddy generates Caddy's JSON config from Ward's DB state and drives
// the Caddy admin API (the config write-path). Ward is the only writer of
// Caddy config; the running config is always derived from the DB.
package caddy

import (
	"encoding/json"
	"fmt"
	"strings"

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

// Generate builds a full Caddy JSON config from services, WAF exclusions, and IP
// blocks. Global blocks short-circuit at the top of the server; per-service
// blocks deny inside that service's route.
func Generate(services []model.Service, exclusions []model.WAFExclusion, blocks []model.BlockedIP, opt Options) ([]byte, error) {
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

	var globalBlocks []string
	blocksByService := map[string][]string{}
	for _, b := range blocks {
		if b.CIDR == "" {
			continue
		}
		switch {
		case b.Scope == "global":
			globalBlocks = append(globalBlocks, b.CIDR)
		case b.ServiceID != nil:
			blocksByService[*b.ServiceID] = append(blocksByService[*b.ServiceID], b.CIDR)
		}
	}

	var internalSubs, managedSubs, skipSubs []string
	routes := make([]any, 0, len(services)+1)
	if len(globalBlocks) > 0 {
		routes = append(routes, denyRoute(globalBlocks)) // first → short-circuits blocked IPs
	}
	for _, s := range services {
		if !s.Enabled || len(s.Upstreams) == 0 {
			continue
		}
		excl := append(append([]string{}, globalExcl...), exclByService[s.ID]...)
		routes = append(routes, serviceRoute(s, opt, excl, blocksByService[s.ID]))
		switch s.TLSMode {
		case "none":
			skipSubs = append(skipSubs, s.PublicHostname)
		case "managed":
			managedSubs = append(managedSubs, s.PublicHostname)
		default: // "internal" or unset → local self-signed CA
			internalSubs = append(internalSubs, s.PublicHostname)
		}
	}

	server := map[string]any{"routes": routes}
	var tlsApp map[string]any
	if opt.DisableAutoHTTPS {
		server["listen"] = []string{opt.HTTPPort}
		server["automatic_https"] = map[string]any{"disable": true}
	} else {
		server["listen"] = []string{opt.HTTPPort, opt.HTTPSPort}
		if len(skipSubs) > 0 {
			server["automatic_https"] = map[string]any{"skip": skipSubs} // HTTP-only services
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
		if len(policies) > 0 {
			tlsApp = map[string]any{"automation": map[string]any{"policies": policies}}
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
	return json.MarshalIndent(cfg, "", "  ")
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

// serviceRoute builds one host-matched route: [per-service deny] → [waf?] → reverse_proxy.
func serviceRoute(s model.Service, opt Options, exclusions, blockCIDRs []string) map[string]any {
	innerRoutes := make([]any, 0, 2)
	if len(blockCIDRs) > 0 {
		innerRoutes = append(innerRoutes, denyRoute(blockCIDRs))
	}

	handlers := make([]any, 0, 2)
	if s.WAFEnabled {
		handlers = append(handlers, wafHandler(opt, exclusions))
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
	return h
}

func wafHandler(opt Options, exclusions []string) map[string]any {
	return map[string]any{
		"handler":        "waf",
		"load_owasp_crs": true,
		"directives":     buildDirectives(opt, exclusions),
	}
}

// buildDirectives is the per-service SecLang: bundled CRS (read-only) → engine
// mode → audit logging → Ward-managed exclusions (appended AFTER the CRS).
func buildDirectives(opt Options, exclusions []string) string {
	mode := opt.WAFEngineMode
	if mode == "" {
		mode = "DetectionOnly"
	}
	audit := opt.AuditLogPath
	if audit == "" {
		audit = "/dev/stdout"
	}
	lines := []string{
		"Include @coraza.conf-recommended",
		"Include @crs-setup.conf.example",
		"Include @owasp_crs/*.conf",
		"SecRuleEngine " + mode,
		"SecAuditEngine On",
		"SecAuditLog " + audit,
		"SecAuditLogType Serial",
		"SecAuditLogFormat JSON",
		"SecAuditLogParts ABIJDEFHZ",
	}
	lines = append(lines, exclusions...)
	return strings.Join(lines, "\n")
}
