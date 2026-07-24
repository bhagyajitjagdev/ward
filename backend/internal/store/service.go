package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

// ErrConflict is returned when a unique constraint (e.g. public_hostname) is violated.
var ErrConflict = errors.New("resource already exists")

// isUniqueViolation reports whether err is a unique-constraint violation on either dialect.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") || // sqlite / modernc
		strings.Contains(s, "duplicate key value") || // postgres
		strings.Contains(s, "23505") // postgres sqlstate
}

// serviceRow is the DB representation. Upstreams is stored as a JSON string so
// the DDL stays dialect-portable (no native array type needed).
type serviceRow struct {
	bun.BaseModel `bun:"table:services,alias:s"`

	ID             string    `bun:"id,pk"`
	Name           string    `bun:"name,notnull"`
	PublicHostname string    `bun:"public_hostname,notnull"` // primary — DB-unique
	ExtraHostnames string    `bun:"extra_hostnames,notnull"` // JSON array of additional hostnames
	HTTPConfig     string    `bun:"http_config,notnull"`     // JSON model.HTTPConfig
	RawCaddy       string    `bun:"raw_caddy,notnull"`       // advanced escape-hatch Caddyfile fragment
	Upstreams      string    `bun:"upstreams,notnull"`
	LBPolicy       string    `bun:"lb_policy,notnull"`
	TLSMode        string    `bun:"tls_mode,notnull"`
	WAFEnabled     bool      `bun:"waf_enabled,notnull"`
	WAFMode        string    `bun:"waf_mode,notnull"`
	Enabled        bool      `bun:"enabled,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull"`
	UpdatedAt      time.Time `bun:"updated_at,notnull"`
}

func (r serviceRow) toModel() (model.Service, error) {
	ups := []string{}
	if r.Upstreams != "" {
		if err := json.Unmarshal([]byte(r.Upstreams), &ups); err != nil {
			return model.Service{}, err
		}
	}
	extras := []string{}
	if r.ExtraHostnames != "" {
		if err := json.Unmarshal([]byte(r.ExtraHostnames), &extras); err != nil {
			return model.Service{}, err
		}
	}
	http := model.HTTPConfig{}
	if r.HTTPConfig != "" {
		if err := json.Unmarshal([]byte(r.HTTPConfig), &http); err != nil {
			return model.Service{}, err
		}
	}
	http.BasicAuthPassword = "" // write-only; never surfaced from storage
	return model.Service{
		ID:              r.ID,
		Name:            r.Name,
		PublicHostname:  r.PublicHostname,
		PublicHostnames: append([]string{r.PublicHostname}, extras...),
		HTTP:            http,
		RawCaddy:        r.RawCaddy,
		Upstreams:       ups,
		LBPolicy:        r.LBPolicy,
		TLSMode:         r.TLSMode,
		WAFEnabled:      r.WAFEnabled,
		WAFMode:         r.WAFMode,
		Enabled:         r.Enabled,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}, nil
}

// marshalHTTP serializes a service's HTTP config for storage, dropping the
// write-only plaintext password (only the bcrypt hash is persisted).
func marshalHTTP(h model.HTTPConfig) (string, error) {
	h.BasicAuthPassword = ""
	b, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// splitHostnames derives the primary hostname + a JSON array of the extras from a
// service's PublicHostnames (falling back to the single PublicHostname alias).
func splitHostnames(in model.Service) (primary, extrasJSON string, err error) {
	names := in.PublicHostnames
	if len(names) == 0 && in.PublicHostname != "" {
		names = []string{in.PublicHostname}
	}
	if len(names) == 0 {
		return "", "", errors.New("at least one hostname is required")
	}
	extras := names[1:]
	if extras == nil {
		extras = []string{}
	}
	b, err := json.Marshal(extras)
	if err != nil {
		return "", "", err
	}
	return names[0], string(b), nil
}

// HostnamesInUse returns which of `names` are already claimed by a service other
// than excludeID (its primary or an extra) — enforces cross-service uniqueness of
// the full hostname set, which no single DB constraint can express here.
func (s *Store) HostnamesInUse(ctx context.Context, names []string, excludeID string) ([]string, error) {
	var rows []serviceRow
	if err := s.DB.NewSelect().Model(&rows).Column("id", "public_hostname", "extra_hostnames").Scan(ctx); err != nil {
		return nil, err
	}
	claimed := map[string]bool{}
	for _, r := range rows {
		if r.ID == excludeID {
			continue
		}
		claimed[strings.ToLower(r.PublicHostname)] = true
		var extras []string
		if r.ExtraHostnames != "" {
			_ = json.Unmarshal([]byte(r.ExtraHostnames), &extras)
		}
		for _, h := range extras {
			claimed[strings.ToLower(strings.TrimSpace(h))] = true
		}
	}
	var dup []string
	for _, n := range names {
		if claimed[strings.ToLower(strings.TrimSpace(n))] {
			dup = append(dup, n)
		}
	}
	return dup, nil
}

// CreateService inserts a new service (server-assigned id + timestamps) and returns it.
func (s *Store) CreateService(ctx context.Context, in model.Service) (model.Service, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.Service{}, err
	}
	ups, err := json.Marshal(orEmpty(in.Upstreams))
	if err != nil {
		return model.Service{}, err
	}
	primary, extras, err := splitHostnames(in)
	if err != nil {
		return model.Service{}, err
	}
	httpCfg, err := marshalHTTP(in.HTTP)
	if err != nil {
		return model.Service{}, err
	}
	now := time.Now().UTC()
	row := serviceRow{
		ID:             id.String(),
		Name:           in.Name,
		PublicHostname: primary,
		ExtraHostnames: extras,
		HTTPConfig:     httpCfg,
		RawCaddy:       in.RawCaddy,
		Upstreams:      string(ups),
		LBPolicy:       orDefault(in.LBPolicy, "round_robin"),
		TLSMode:        orDefault(in.TLSMode, "internal"),
		WAFEnabled:     in.WAFEnabled,
		WAFMode:        in.WAFMode,
		Enabled:        true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, err := s.DB.NewInsert().Model(&row).Exec(ctx); err != nil {
		if isUniqueViolation(err) {
			return model.Service{}, ErrConflict
		}
		return model.Service{}, err
	}
	return row.toModel()
}

// ListServices returns all services, newest first.
func (s *Store) ListServices(ctx context.Context) ([]model.Service, error) {
	var rows []serviceRow
	if err := s.DB.NewSelect().Model(&rows).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.Service, 0, len(rows))
	for _, r := range rows {
		m, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// GetService returns one service by id (ErrNotFound if missing).
func (s *Store) GetService(ctx context.Context, id string) (model.Service, error) {
	var row serviceRow
	err := s.DB.NewSelect().Model(&row).Where("id = ?", id).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Service{}, ErrNotFound
	}
	if err != nil {
		return model.Service{}, err
	}
	return row.toModel()
}

// UpdateService replaces a service's mutable fields and returns the updated row
// (ErrNotFound if missing, ErrConflict on a hostname clash).
func (s *Store) UpdateService(ctx context.Context, id string, in model.Service) (model.Service, error) {
	ups, err := json.Marshal(orEmpty(in.Upstreams))
	if err != nil {
		return model.Service{}, err
	}
	primary, extras, err := splitHostnames(in)
	if err != nil {
		return model.Service{}, err
	}
	httpCfg, err := marshalHTTP(in.HTTP)
	if err != nil {
		return model.Service{}, err
	}
	row := serviceRow{
		ID:             id,
		Name:           in.Name,
		PublicHostname: primary,
		ExtraHostnames: extras,
		HTTPConfig:     httpCfg,
		RawCaddy:       in.RawCaddy,
		Upstreams:      string(ups),
		LBPolicy:       orDefault(in.LBPolicy, "round_robin"),
		TLSMode:        orDefault(in.TLSMode, "internal"),
		WAFEnabled:     in.WAFEnabled,
		WAFMode:        in.WAFMode,
		Enabled:        in.Enabled,
		UpdatedAt:      time.Now().UTC(),
	}
	res, err := s.DB.NewUpdate().Model(&row).
		Column("name", "public_hostname", "extra_hostnames", "http_config", "raw_caddy", "upstreams", "lb_policy", "tls_mode", "waf_enabled", "waf_mode", "enabled", "updated_at").
		WherePK().Exec(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Service{}, ErrConflict
		}
		return model.Service{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Service{}, ErrNotFound
	}
	return s.GetService(ctx, id)
}

// DeleteService removes a service; reports whether one was found.
func (s *Store) DeleteService(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.NewDelete().Model((*serviceRow)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
