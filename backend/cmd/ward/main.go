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

	"github.com/bhagyajitjagdev/ward/backend/internal/api"
	"github.com/bhagyajitjagdev/ward/backend/internal/caddy"
	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
	"github.com/bhagyajitjagdev/ward/backend/internal/waf"
)

func main() {
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
		cfg, err := caddy.Generate(caddy.Input{
			Services:   services,
			Exclusions: exclusions,
			Blocks:     blocks,
			RateLimits: rateLimits,
			GeoRules:   geoRules,
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

	// Startup reconcile: push current DB state to Caddy. Non-fatal — the control
	// plane runs independently of the edge (principle #1).
	if err := applier.Apply(context.Background()); err != nil {
		log.Printf("startup reconcile: could not push config to Caddy (continuing): %v", err)
	} else {
		log.Printf("startup reconcile: pushed config to Caddy")
	}

	// WAF read-path: tail Coraza's JSON audit log into waf_events.
	if auditPath := os.Getenv("WARD_AUDIT_LOG"); auditPath != "" {
		ing := waf.NewIngester(st, auditPath)
		if ms, err := strconv.Atoi(os.Getenv("WARD_AUDIT_INTERVAL_MS")); err == nil && ms > 0 {
			ing.SetInterval(time.Duration(ms) * time.Millisecond)
		}
		go ing.Run(context.Background())
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(st, applier).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("ward listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func caddyOptions() caddy.Options {
	opt := caddy.DefaultOptions()
	if v := os.Getenv("WARD_CADDY_ADMIN_LISTEN"); v != "" {
		opt.AdminListen = v
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
