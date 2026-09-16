package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// DesiredState is the whole declarative edge config the DB holds — everything the
// Caddy config is generated from. A config snapshot stores it next to the rendered
// Caddy JSON, so a rollback restores the *source of truth* (the DB) and then
// regenerates; a live-edge-only restore would be overwritten by the drift
// reconciler on its next tick (architecture §5 + §7).
type DesiredState struct {
	Services    []model.Service       `json:"services"`
	Exclusions  []model.WAFExclusion  `json:"waf_exclusions"`
	CustomRules []model.WAFCustomRule `json:"waf_custom_rules"`
	Blocks      []model.BlockedIP     `json:"blocklist"` // all, incl. expired — the renderer filters
	RateLimits  []model.RateLimit     `json:"rate_limits"`
	GeoRules    []model.GeoRule       `json:"geo_rules"`
	Trusted     []model.TrustedIP     `json:"trusted_ips"`
	// Settings holds the config-shaping keys only (ConfigSettingKeys); "" = unset,
	// which the renderer resolves to its env/compiled default exactly as the live
	// path does.
	Settings map[string]string `json:"settings"`
}

// ConfigSettingKeys are the settings that shape the generated config, and so are
// captured in snapshots and restored on rollback. Operational keys — retention
// windows, ingest offsets, the GeoIP source/key, the exclusion-id counter — are
// deliberately left alone by a rollback.
var ConfigSettingKeys = []string{
	WAFModeKey, ACMEEmailKey, CrowdSecEnabledKey, MetricsEnabledKey,
	LogLevelKey, AccessLogErrorsKey, TLSMinVersionKey,
}

// LoadDesiredState reads the full declarative state from the DB.
func (s *Store) LoadDesiredState(ctx context.Context) (DesiredState, error) {
	var ds DesiredState
	var err error
	if ds.Services, err = s.ListServices(ctx); err != nil {
		return ds, err
	}
	if ds.Exclusions, err = s.ListExclusions(ctx); err != nil {
		return ds, err
	}
	if ds.CustomRules, err = s.ListWAFCustomRules(ctx); err != nil {
		return ds, err
	}
	if ds.Blocks, err = s.ListBlocks(ctx); err != nil {
		return ds, err
	}
	if ds.RateLimits, err = s.ListRateLimits(ctx); err != nil {
		return ds, err
	}
	if ds.GeoRules, err = s.ListGeoRules(ctx); err != nil {
		return ds, err
	}
	if ds.Trusted, err = s.ListTrusted(ctx); err != nil {
		return ds, err
	}
	ds.Settings = map[string]string{}
	for _, k := range ConfigSettingKeys {
		v, err := s.GetSetting(ctx, k)
		if err != nil {
			return ds, err
		}
		ds.Settings[k] = v
	}
	return ds, nil
}

// ParseDesiredState decodes a snapshot's stored state.
func ParseDesiredState(raw []byte) (DesiredState, error) {
	var ds DesiredState
	if len(raw) == 0 {
		return ds, errors.New("empty desired state")
	}
	err := json.Unmarshal(raw, &ds)
	return ds, err
}

// RestoreDesiredState makes the DB match ds, in one transaction: every row in ds
// is upserted by id (so ids, timestamps and — for services — the waf/access
// events that reference them survive), and rows absent from ds are deleted.
// Services go first because the other tables reference them.
func (s *Store) RestoreDesiredState(ctx context.Context, ds DesiredState) error {
	return s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		ids := make([]string, 0, len(ds.Services))
		for _, m := range ds.Services {
			row, err := serviceRowFrom(m)
			if err != nil {
				return err
			}
			row.ID, row.Enabled, row.CreatedAt, row.UpdatedAt = m.ID, m.Enabled, m.CreatedAt, m.UpdatedAt
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*serviceRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.Exclusions {
			row := wafExclusionRow{
				ID: m.ID, Scope: orDefault(m.Scope, "service"), ServiceID: m.ServiceID, RuleID: m.RuleID,
				Path: m.Path, PathMatch: orDefault(m.PathMatch, "prefix"), Methods: joinMethods(m.Methods),
				Target: m.Target, SecLang: m.SecLang, State: orDefault(m.State, "active"),
				Source: orDefault(m.Source, "manual"), CreatedAt: m.CreatedAt,
			}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*wafExclusionRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.CustomRules {
			row := wafCustomRuleRow{
				ID: m.ID, Scope: orDefault(m.Scope, "global"), ServiceID: m.ServiceID, Name: m.Name,
				SecLang: m.SecLang, Enabled: m.Enabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
			}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*wafCustomRuleRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.Blocks {
			row := blockRow{
				ID: m.ID, Scope: orDefault(m.Scope, "global"), Mode: orDefault(m.Mode, "block"),
				ServiceID: m.ServiceID, CIDR: m.CIDR, Reason: m.Reason, Source: orDefault(m.Source, "manual"),
				ExpiresAt: m.ExpiresAt, CreatedAt: m.CreatedAt,
			}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*blockRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.RateLimits {
			row := rateLimitRow{
				ID: m.ID, Scope: orDefault(m.Scope, "global"), ServiceID: m.ServiceID,
				MaxEvents: m.MaxEvents, Window: m.Window, CreatedAt: m.CreatedAt,
			}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*rateLimitRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.GeoRules {
			cs, err := json.Marshal(orEmpty(m.Countries))
			if err != nil {
				return err
			}
			row := geoRuleRow{
				ID: m.ID, Scope: orDefault(m.Scope, "global"), Mode: orDefault(m.Mode, "block"),
				ServiceID: m.ServiceID, Countries: string(cs), CreatedAt: m.CreatedAt,
			}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*geoRuleRow)(nil), ids); err != nil {
			return err
		}

		ids = ids[:0]
		for _, m := range ds.Trusted {
			row := trustedRow{ID: m.ID, CIDR: m.CIDR, Note: m.Note, CreatedAt: m.CreatedAt}
			if err := upsert(ctx, tx, &row); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		if err := pruneExcept(ctx, tx, (*trustedRow)(nil), ids); err != nil {
			return err
		}

		// Settings: a key set in the snapshot is upserted; one unset there is removed
		// so it falls back to the env/compiled default, as it did at the time.
		for _, k := range ConfigSettingKeys {
			v := ds.Settings[k]
			if v == "" {
				if _, err := tx.NewDelete().Model((*settingRow)(nil)).Where("key = ?", k).Exec(ctx); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.NewInsert().Model(&settingRow{Key: k, Value: v}).
				On("CONFLICT (key) DO UPDATE").Set("value = EXCLUDED.value").Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// upsert writes one row by primary key: update if it exists, else insert. Two
// statements instead of ON CONFLICT so no per-table column list is needed; the
// config tables are tiny.
func upsert(ctx context.Context, tx bun.Tx, row any) error {
	res, err := tx.NewUpdate().Model(row).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = tx.NewInsert().Model(row).Exec(ctx)
	return err
}

// pruneExcept deletes every row of model's table whose id is not in keep.
func pruneExcept(ctx context.Context, tx bun.Tx, model any, keep []string) error {
	q := tx.NewDelete().Model(model)
	if len(keep) == 0 {
		q = q.Where("1 = 1")
	} else {
		q = q.Where("id NOT IN (?)", bun.In(keep))
	}
	_, err := q.Exec(ctx)
	return err
}

// ensure time is referenced (row timestamps come from the snapshot verbatim).
var _ = time.Time{}
