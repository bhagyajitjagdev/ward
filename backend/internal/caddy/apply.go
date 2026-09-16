package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// ErrNotRestorable is returned by Rollback for a snapshot taken before Ward began
// recording the declarative state next to the rendered config. Such a snapshot
// holds only the Caddy JSON; re-loading that alone would be undone by the next
// reconcile, so it is refused rather than pretending.
var ErrNotRestorable = errors.New("snapshot holds only the rendered Caddy config, not the declarative state it came from, so it can't be restored — snapshots taken since this Ward version are")

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

// NewApplier wires an Applier over the store + Caddy admin client. client may be
// nil for render-only use (`ward gen-config`).
func NewApplier(s *store.Store, c *Client, opt Options) *Applier {
	return &Applier{store: s, client: c, opt: opt}
}

// applySettings overlays the DB settings captured in a DesiredState onto the
// env/compiled options — the same precedence the store getters use (an unset key
// keeps the default).
func applySettings(opt *Options, st map[string]string) {
	if v := st[store.WAFModeKey]; v != "" {
		opt.WAFEngineMode = v
	}
	if v := st[store.ACMEEmailKey]; v != "" {
		opt.ACMEEmail = v
	}
	if v := st[store.CrowdSecEnabledKey]; v != "" {
		opt.CrowdSecEnabled = v == "1"
	}
	if v := st[store.MetricsEnabledKey]; v != "" {
		opt.MetricsEnabled = v == "1"
	}
	if v := st[store.LogLevelKey]; v != "" {
		opt.LogLevel = v
	}
	if v := st[store.AccessLogErrorsKey]; v != "" {
		opt.AccessLogErrorsOnly = v == "1"
	}
	if v := st[store.TLSMinVersionKey]; v != "" {
		opt.TLSMinVersion = v
	}
}

// activeBlocks drops expired entries — a snapshot keeps every block so a restore is
// complete, but only live ones reach the edge.
func activeBlocks(blocks []model.BlockedIP, now time.Time) []model.BlockedIP {
	out := make([]model.BlockedIP, 0, len(blocks))
	for _, b := range blocks {
		if b.ExpiresAt == nil || b.ExpiresAt.After(now) {
			out = append(out, b)
		}
	}
	return out
}

// render turns a desired state into the Caddy config for it. The only inputs
// beyond ds are environment facts: the options, the GeoIP file, the certs volume,
// and the caddy binary for raw fragments.
func (a *Applier) render(ds store.DesiredState) ([]byte, error) {
	opt := a.opt
	opt.GeoIPDBPath = geoip.ActivePath(geoip.Dir()) // pick up a newly added/removed DB
	applySettings(&opt, ds.Settings)
	return Generate(Input{
		Services:     ds.Services,
		Exclusions:   ds.Exclusions,
		CustomRules:  ds.CustomRules,
		Blocks:       activeBlocks(ds.Blocks, time.Now().UTC()),
		RateLimits:   ds.RateLimits,
		GeoRules:     ds.GeoRules,
		Trusted:      ds.Trusted,
		Certificates: ResolveCustomCerts(),
		RawRoutes:    AdaptRawRoutes(ds.Services),
	}, opt)
}

// Render returns the config Ward would push right now, without touching the edge
// (`ward gen-config`).
func (a *Applier) Render(ctx context.Context) ([]byte, error) {
	ds, err := a.store.LoadDesiredState(ctx)
	if err != nil {
		return nil, err
	}
	return a.render(ds)
}

// Apply reads desired state from the DB, generates the full Caddy config, loads
// it (Caddy validates + applies atomically), and snapshots the applied config
// together with the state it came from.
func (a *Applier) Apply(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ds, err := a.store.LoadDesiredState(ctx)
	if err != nil {
		return err
	}
	cfg, err := a.render(ds)
	if err != nil {
		return err
	}
	if err := a.client.Load(ctx, cfg); err != nil {
		return err // Caddy kept the previous config; do not snapshot
	}
	dsJSON, err := json.Marshal(ds)
	if err != nil {
		return err
	}
	return a.store.SaveSnapshot(ctx, cfg, dsJSON, "")
}

// Rollback restores the DB to the desired state a snapshot was taken from, then
// pushes the regenerated config — so the rollback *is* the new source of truth and
// survives the drift reconciler. Order: render → load (Caddy validates; nothing has
// changed if it refuses) → restore the DB → record a new active snapshot. Env-level
// inputs (the GeoIP file, uploaded certs, the LAPI key) are whatever they are now;
// the snapshot only owns what the DB owns.
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
	if snap.WardJSON == "" {
		return ErrNotRestorable
	}
	ds, err := store.ParseDesiredState([]byte(snap.WardJSON))
	if err != nil {
		return err
	}
	cfg, err := a.render(ds)
	if err != nil {
		return err
	}
	if err := a.client.Load(ctx, cfg); err != nil {
		return err
	}
	if err := a.store.RestoreDesiredState(ctx, ds); err != nil {
		return err
	}
	dsJSON, err := json.Marshal(ds)
	if err != nil {
		return err
	}
	return a.store.SaveSnapshot(ctx, cfg, dsJSON, "rollback to "+snapshotID)
}
