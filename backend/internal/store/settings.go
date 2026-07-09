package store

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/uptrace/bun"
)

type settingRow struct {
	bun.BaseModel `bun:"table:settings,alias:st"`

	Key   string `bun:"key,pk"`
	Value string `bun:"value,notnull"`
}

// Settings keys.
const (
	WAFModeKey         = "waf.engine_mode"       // global WAF engine-mode default
	ACMEEmailKey       = "acme.email"            // contact email for managed (Let's Encrypt) certs
	AccessRetentionKey = "access.retention_days" // how long to keep raw access events
)

// WAFEngineMode returns the global WAF engine-mode default, falling back to
// `fallback` (the env/compiled default) when the setting is unset.
func (s *Store) WAFEngineMode(ctx context.Context, fallback string) string {
	if v, err := s.GetSetting(ctx, WAFModeKey); err == nil && v != "" {
		return v
	}
	return fallback
}

// ACMEEmail returns the managed-cert contact email, falling back to `fallback`
// (the env default) when unset.
func (s *Store) ACMEEmail(ctx context.Context, fallback string) string {
	if v, err := s.GetSetting(ctx, ACMEEmailKey); err == nil && v != "" {
		return v
	}
	return fallback
}

// AccessRetentionDays returns how many days of raw access events to keep, falling
// back to `fallback` when unset.
func (s *Store) AccessRetentionDays(ctx context.Context, fallback int) int {
	if v, err := s.GetSetting(ctx, AccessRetentionKey); err == nil && v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// GetSetting returns the value for key, or "" if unset.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var row settingRow
	err := s.DB.NewSelect().Model(&row).Where("key = ?", key).Limit(1).Scan(ctx)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Value, nil
}

// SetSetting upserts a key/value (dialect-portable via bun's ON CONFLICT).
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	row := &settingRow{Key: key, Value: value}
	_, err := s.DB.NewInsert().Model(row).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Exec(ctx)
	return err
}

// NextSeq atomically increments a named counter and returns the new value
// (starting at `start` on first use). Used to assign reserved SecLang rule ids.
func (s *Store) NextSeq(ctx context.Context, key string, start int64) (int64, error) {
	var next int64
	err := s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var row settingRow
		e := tx.NewSelect().Model(&row).Where("key = ?", key).Limit(1).Scan(ctx)
		switch {
		case e == sql.ErrNoRows:
			next = start
		case e != nil:
			return e
		default:
			cur, _ := strconv.ParseInt(row.Value, 10, 64)
			next = cur + 1
		}
		_, err := tx.NewInsert().Model(&settingRow{Key: key, Value: strconv.FormatInt(next, 10)}).
			On("CONFLICT (key) DO UPDATE").
			Set("value = EXCLUDED.value").
			Exec(ctx)
		return err
	})
	return next, err
}
