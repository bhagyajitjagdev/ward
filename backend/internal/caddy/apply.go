package caddy

import (
	"context"
	"sync"

	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

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
	opt.GeoIPDBPath = geoip.ActivePath(geoip.Dir())               // pick up a newly added/removed DB
	opt.WAFEngineMode = a.store.WAFEngineMode(ctx, opt.WAFEngineMode) // DB setting overrides the env/compiled default
	opt.ACMEEmail = a.store.ACMEEmail(ctx, opt.ACMEEmail)
	cfg, err := Generate(Input{
		Services:     services,
		Exclusions:   exclusions,
		Blocks:       blocks,
		RateLimits:   rateLimits,
		GeoRules:     geoRules,
		Certificates: ResolveCustomCerts(),
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
