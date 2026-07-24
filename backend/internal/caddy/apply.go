package caddy

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// AdaptRawRoutes adapts each service's raw_caddy fragment into route objects. A
// fragment that fails to adapt is skipped (logged) rather than breaking the whole
// edge — it was validated at save, so this only trips on a broken environment.
func AdaptRawRoutes(services []model.Service) map[string][]json.RawMessage {
	out := map[string][]json.RawMessage{}
	for _, s := range services {
		if s.RawCaddy == "" {
			continue
		}
		rr, err := AdaptFragment(s.RawCaddy)
		if err != nil {
			log.Printf("reconcile: skipping raw_caddy for service %s (adapt failed): %v", s.ID, err)
			continue
		}
		out[s.ID] = rr
	}
	return out
}

// Applier owns the config write-path: regenerate from the DB → validate+load via
// the admin API → snapshot. Serialized so concurrent applies can't clobber.
type Applier struct {
	store  *store.Store
	client *Client
	opt    Options
	mu     sync.Mutex
}

// NewApplier wires an Applier over the store + Caddy admin client.
func NewApplier(s *store.Store, c *Client, opt Options) *Applier {
	return &Applier{store: s, client: c, opt: opt}
}

// Apply reads desired state from the DB, generates the full Caddy config, loads
// it (Caddy validates + applies atomically), and snapshots the applied config.
func (a *Applier) Apply(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	services, err := a.store.ListServices(ctx)
	if err != nil {
		return err
	}
	exclusions, err := a.store.ListExclusions(ctx)
	if err != nil {
		return err
	}
	customRules, err := a.store.ListWAFCustomRules(ctx)
	if err != nil {
		return err
	}
	blocks, err := a.store.ListActiveBlocks(ctx)
	if err != nil {
		return err
	}
	rateLimits, err := a.store.ListRateLimits(ctx)
	if err != nil {
		return err
	}
	geoRules, err := a.store.ListGeoRules(ctx)
	if err != nil {
		return err
	}
	opt := a.opt
	opt.GeoIPDBPath = geoip.ActivePath(geoip.Dir())                   // pick up a newly added/removed DB
	opt.WAFEngineMode = a.store.WAFEngineMode(ctx, opt.WAFEngineMode) // DB setting overrides the env/compiled default
	opt.ACMEEmail = a.store.ACMEEmail(ctx, opt.ACMEEmail)
	opt.CrowdSecEnabled = a.store.CrowdSecEnabled(ctx, opt.CrowdSecEnabled) // DB toggle over the env default (URL+key stay env)
	cfg, err := Generate(Input{
		Services:     services,
		Exclusions:   exclusions,
		CustomRules:  customRules,
		Blocks:       blocks,
		RateLimits:   rateLimits,
		GeoRules:     geoRules,
		Certificates: ResolveCustomCerts(),
		RawRoutes:    AdaptRawRoutes(services),
	}, opt)
	if err != nil {
		return err
	}
	if err := a.client.Load(ctx, cfg); err != nil {
		return err // Caddy kept the previous config; do not snapshot
	}
	return a.store.SaveSnapshot(ctx, cfg)
}

// Rollback re-loads a stored snapshot into Caddy and marks it active. Note: the
// DB stays the source of truth, so the next Apply regenerates from current DB
// state — rollback is a live-edge restore, not a DB undo.
func (a *Applier) Rollback(ctx context.Context, snapshotID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	snap, err := a.store.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}
	if snap == nil {
		return store.ErrNotFound
	}
	if err := a.client.Load(ctx, []byte(snap.CaddyJSON)); err != nil {
		return err
	}
	return a.store.SetActiveSnapshot(ctx, snapshotID)
}
