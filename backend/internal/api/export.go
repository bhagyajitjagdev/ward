package api

import "net/http"

// exportConfig returns Ward's declarative config (the DB's truth) as portable JSON —
// for backup, git-review, and remote diffing. Read-only; basic-auth hashes are blanked
// and no timestamp is emitted, so the output is stable/diffable. Import is not offered
// yet (see planned.md).
func (h *Handler) exportConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	services, err := h.store.ListServices(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	for i := range services {
		services[i] = sanitizeService(services[i])
	}
	exclusions, err := h.store.ListExclusions(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	rules, err := h.store.ListWAFCustomRules(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	blocks, err := h.store.ListBlocks(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	rateLimits, err := h.store.ListRateLimits(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	geoRules, err := h.store.ListGeoRules(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": 1,
		"settings": map[string]any{
			"waf_engine_mode":       h.store.WAFEngineMode(ctx, "DetectionOnly"),
			"acme_email":            h.store.ACMEEmail(ctx, ""),
			"access_retention_days": h.store.AccessRetentionDays(ctx, 7),
			"waf_retention_days":    h.store.WAFRetentionDays(ctx, 30),
			"crowdsec_enabled":      h.store.CrowdSecEnabled(ctx, h.crowdsec != nil),
		},
		"services":         services,
		"waf_exclusions":   exclusions,
		"waf_custom_rules": rules,
		"blocklist":        blocks,
		"rate_limits":      rateLimits,
		"geo_rules":        geoRules,
	})
}
