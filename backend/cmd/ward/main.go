// Command ward is the Ward control-plane server.
//
//	ward              run the API server + reconcile the edge
//	ward gen-config   print the Caddy config Ward would generate, then exit
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/access"
	"github.com/bhagyajitjagdev/ward/backend/internal/api"
	"github.com/bhagyajitjagdev/ward/backend/internal/caddy"
	"github.com/bhagyajitjagdev/ward/backend/internal/crowdsec"
	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
	"github.com/bhagyajitjagdev/ward/backend/internal/waf"
	"github.com/bhagyajitjagdev/ward/backend/internal/web"
)

// version is the Ward build version, injected at build time via
// -ldflags "-X main.version=…" (see backend/Dockerfile). "dev" for local builds.
// The sidebar shows it and checks GitHub (client-side) for a newer release.
var version = "dev"

func main() {
	api.Version = version
	dsn := env("WARD_DB", "file:ward.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	addr := env("WARD_ADDR", ":8080")

	st, err := store.Open(dsn)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	// `ward gen-config` — print the generated config and exit (validation / ops aid).
	if len(os.Args) > 1 && os.Args[1] == "gen-config" {
		ctx := context.Background()
		services, err := st.ListServices(ctx)
		if err != nil {
			log.Fatal(err)
		}
		exclusions, err := st.ListExclusions(ctx)
		if err != nil {
			log.Fatal(err)
		}
		customRules, err := st.ListWAFCustomRules(ctx)
		if err != nil {
			log.Fatal(err)
		}
		blocks, err := st.ListActiveBlocks(ctx)
		if err != nil {
			log.Fatal(err)
		}
		rateLimits, err := st.ListRateLimits(ctx)
		if err != nil {
			log.Fatal(err)
		}
		geoRules, err := st.ListGeoRules(ctx)
		if err != nil {
			log.Fatal(err)
		}
		opt := caddyOptions()
		opt.WAFEngineMode = st.WAFEngineMode(ctx, opt.WAFEngineMode)
		opt.ACMEEmail = st.ACMEEmail(ctx, opt.ACMEEmail)
		opt.CrowdSecEnabled = st.CrowdSecEnabled(ctx, opt.CrowdSecEnabled)
		cfg, err := caddy.Generate(caddy.Input{
			Services:     services,
			Exclusions:   exclusions,
			CustomRules:  customRules,
			Blocks:       blocks,
			RateLimits:   rateLimits,
			GeoRules:     geoRules,
			Certificates: caddy.ResolveCustomCerts(),
			RawRoutes:    caddy.AdaptRawRoutes(services),
		}, opt)
		if err != nil {
			log.Fatal(err)
		}
		_, _ = os.Stdout.Write(cfg)
		return
	}

	log.Printf("store ready (dialect=%s); migrations applied", st.Dialect())

	client := caddy.NewClient(env("WARD_CADDY_ADMIN", "http://localhost:2019"))
	applier := caddy.NewApplier(st, client, caddyOptions())

	// Reconcile the edge in the background: retry until Caddy's admin answers (a cold
	// `compose up` races it), then re-push periodically so drift (a Caddy restart or a
	// lost config) self-heals. The control plane never blocks on the edge (principle #1).
	// See architecture.md §7 (reconciliation loop — bootstrap + drift).
	go reconcileLoop(context.Background(), applier)

	// WAF read-path: tail Coraza's JSON audit log into waf_events.
	if auditPath := os.Getenv("WARD_AUDIT_LOG"); auditPath != "" {
		ing := waf.NewIngester(st, auditPath)
		if ms, err := strconv.Atoi(os.Getenv("WARD_AUDIT_INTERVAL_MS")); err == nil && ms > 0 {
			ing.SetInterval(time.Duration(ms) * time.Millisecond)
		}
		go ing.Run(context.Background())
	}

	// Access-log read-path: tail Caddy's JSON access log into access_events.
	if accessPath := os.Getenv("WARD_ACCESS_LOG"); accessPath != "" {
		ing := access.NewIngester(st, accessPath)
		if ms, err := strconv.Atoi(os.Getenv("WARD_ACCESS_INTERVAL_MS")); err == nil && ms > 0 {
			ing.SetInterval(time.Duration(ms) * time.Millisecond)
		}
		go ing.Run(context.Background())
	}

	// Prune access + WAF events to their retention windows (defaults 7d / 30d),
	// hourly. Runs regardless of ingestion — a no-op on empty tables, and the
	// windows still apply if ingestion is enabled later.
	go pruneLoop(context.Background(), st)

	// The API lives under /api; the embedded ward-ui (when compiled in and
	// WARD_UI != "0") serves the SPA at everything else, on the same private port.
	// Read-only CrowdSec LAPI client (nil unless the deployment set the URL + key).
	var csClient *crowdsec.Client
	if u, k := os.Getenv("WARD_CROWDSEC_API_URL"), os.Getenv("WARD_CROWDSEC_API_KEY"); u != "" && k != "" {
		csClient = crowdsec.New(u, k)
	}

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api.New(st, applier).WithCrowdSec(csClient).Routes()))
	if os.Getenv("WARD_UI") != "0" {
		if assets, ok := web.Assets(); ok {
			root.Handle("/", web.SPAHandler(assets))
			log.Printf("serving embedded ward-ui at /")
		}
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("ward listening on %s (API under /api)", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func caddyOptions() caddy.Options {
	opt := caddy.DefaultOptions()
	if v := os.Getenv("WARD_CADDY_ADMIN_LISTEN"); v != "" {
		opt.AdminListen = v
	}
	// Override the edge's HTTP/HTTPS listen ports (default :80/:443) — for dev or a
	// rootless host that can't bind privileged ports.
	if v := os.Getenv("WARD_HTTP_PORT"); v != "" {
		opt.HTTPPort = v
	}
	if v := os.Getenv("WARD_HTTPS_PORT"); v != "" {
		opt.HTTPSPort = v
	}
	if v := os.Getenv("WARD_WAF_ENGINE"); v != "" {
		opt.WAFEngineMode = v
	}
	if v := os.Getenv("WARD_WAF_AUDIT_LOG"); v != "" {
		opt.AuditLogPath = v
	}
	if v := os.Getenv("WARD_ACME_EMAIL"); v != "" {
		opt.ACMEEmail = v
	}
	opt.AccessLogPath = os.Getenv("WARD_ACCESS_LOG")
	// CrowdSec LAPI coordinates are deployment secrets (compose env), not DB settings.
	// Default enabled when both are present; a DB toggle can still turn it off.
	opt.CrowdSecAPIURL = os.Getenv("WARD_CROWDSEC_API_URL")
	opt.CrowdSecAPIKey = os.Getenv("WARD_CROWDSEC_API_KEY")
	opt.CrowdSecEnabled = opt.CrowdSecAPIURL != "" && opt.CrowdSecAPIKey != ""
	opt.GeoIPDBPath = geoip.ActivePath(geoip.Dir())
	// Auto-HTTPS off by default in dev (no public domains → no ACME); opt in with =1.
	opt.DisableAutoHTTPS = os.Getenv("WARD_CADDY_AUTO_HTTPS") != "1"
	return opt
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// pruneLoop deletes access events and WAF detections past their retention
// windows, hourly. WAF events default to a longer window — they're the tuning
// signal (Top Triggers, exclusion decisions), not just traffic history.
func pruneLoop(ctx context.Context, st *store.Store) {
	prune := func() {
		now := time.Now().UTC()
		days := st.AccessRetentionDays(ctx, 7)
		if n, err := st.PruneAccessEvents(ctx, now.AddDate(0, 0, -days)); err == nil && n > 0 {
			log.Printf("access pruner: removed %d events older than %dd", n, days)
		}
		days = st.WAFRetentionDays(ctx, 30)
		if n, err := st.PruneWAFEvents(ctx, now.AddDate(0, 0, -days)); err == nil && n > 0 {
			log.Printf("waf pruner: removed %d detections older than %dd", n, days)
		}
	}
	prune()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// reconcileLoop keeps Caddy's config in sync with the DB. It first retries with
// backoff until the edge accepts a push (Ward can win the startup race against
// Caddy's admin), then re-applies on an interval so a Caddy restart or a lost
// config self-heals. Never blocks the control plane (principle #1).
func reconcileLoop(ctx context.Context, applier *caddy.Applier) {
	for delay := time.Second; ; {
		if err := applier.Apply(ctx); err == nil {
			log.Printf("reconcile: pushed config to Caddy")
			break
		} else {
			log.Printf("reconcile: edge not ready, retrying in %s (%v)", delay, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 15*time.Second {
			delay *= 2
		}
	}
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := applier.Apply(ctx); err != nil {
				log.Printf("reconcile (drift): %v", err)
			}
		}
	}
}
