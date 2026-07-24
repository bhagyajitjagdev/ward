package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type wafEventRow struct {
	bun.BaseModel `bun:"table:waf_events,alias:we"`

	ID             string    `bun:"id,pk"`
	TxID           string    `bun:"tx_id,notnull"`
	TS             time.Time `bun:"ts,notnull"`
	ServiceID      *string   `bun:"service_id"`
	Host           string    `bun:"host"`
	ClientIP       string    `bun:"client_ip"`
	Authed         bool      `bun:"authed,notnull"`
	UserAgent      string    `bun:"user_agent"`
	Method         string    `bun:"method"`
	Path           string    `bun:"path,notnull"`
	URI            string    `bun:"uri"`
	Status         int       `bun:"status"`
	EngineMode     string    `bun:"engine_mode"`
	IsInterrupted  bool      `bun:"is_interrupted,notnull"`
	RuleID         int       `bun:"rule_id"`
	RuleMsg        string    `bun:"rule_msg"`
	Severity       string    `bun:"severity"`
	MatchedTarget  string    `bun:"matched_target"`
	MatchedValue   string    `bun:"matched_value"`
	Tags           string    `bun:"tags"` // JSON array
	IsAnomalyScore bool      `bun:"is_anomaly_score,notnull"`
	CRSVersion     string    `bun:"crs_version"`
	Raw            string    `bun:"raw"`
}

func (r wafEventRow) toModel() model.WAFEvent {
	tags := []string{}
	if r.Tags != "" {
		_ = json.Unmarshal([]byte(r.Tags), &tags)
	}
	return model.WAFEvent{
		ID: r.ID, TxID: r.TxID, TS: r.TS, ServiceID: r.ServiceID, Host: r.Host,
		ClientIP: r.ClientIP, Authed: r.Authed, UserAgent: r.UserAgent, Method: r.Method,
		Path: r.Path, URI: r.URI, Status: r.Status, EngineMode: r.EngineMode,
		IsInterrupted: r.IsInterrupted, RuleID: r.RuleID, RuleMsg: r.RuleMsg,
		Severity: r.Severity, MatchedTarget: r.MatchedTarget, MatchedValue: r.MatchedValue,
		Tags: tags, IsAnomalyScore: r.IsAnomalyScore, CRSVersion: r.CRSVersion, Raw: r.Raw,
	}
}

// InsertWAFEvents batch-inserts parsed events (server-assigned ids).
func (s *Store) InsertWAFEvents(ctx context.Context, events []model.WAFEvent) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]wafEventRow, 0, len(events))
	for _, e := range events {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		tags, _ := json.Marshal(orEmpty(e.Tags))
		rows = append(rows, wafEventRow{
			ID: id.String(), TxID: e.TxID, TS: e.TS, ServiceID: e.ServiceID, Host: e.Host,
			ClientIP: e.ClientIP, Authed: e.Authed, UserAgent: e.UserAgent, Method: e.Method,
			Path: e.Path, URI: e.URI, Status: e.Status, EngineMode: e.EngineMode,
			IsInterrupted: e.IsInterrupted, RuleID: e.RuleID, RuleMsg: e.RuleMsg,
			Severity: e.Severity, MatchedTarget: e.MatchedTarget, MatchedValue: e.MatchedValue,
			Tags: string(tags), IsAnomalyScore: e.IsAnomalyScore, CRSVersion: e.CRSVersion, Raw: e.Raw,
		})
	}
	_, err := s.DB.NewInsert().Model(&rows).Exec(ctx)
	return err
}

// WAFEventFilter narrows a ListWAFEvents query.
type WAFEventFilter struct {
	ServiceID string
	Path      string
	RuleID    int
	ClientIP  string
	Since     time.Time
	Limit     int
}

// ListWAFEvents returns matching events, newest first (default/max limit 100/500).
func (s *Store) ListWAFEvents(ctx context.Context, f WAFEventFilter) ([]model.WAFEvent, error) {
	var rows []wafEventRow
	q := s.DB.NewSelect().Model(&rows).Order("ts DESC")
	if f.ServiceID != "" {
		q = q.Where("service_id = ?", f.ServiceID)
	}
	if f.Path != "" {
		q = q.Where("path = ?", f.Path)
	}
	if f.RuleID != 0 {
		q = q.Where("rule_id = ?", f.RuleID)
	}
	if f.ClientIP != "" {
		q = q.Where("client_ip = ?", f.ClientIP)
	}
	if !f.Since.IsZero() {
		q = q.Where("ts >= ?", f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if err := q.Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.WAFEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// PruneWAFEvents deletes detections older than `before`; returns rows removed.
func (s *Store) PruneWAFEvents(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.DB.NewDelete().Model((*wafEventRow)(nil)).Where("ts < ?", before).Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// LatestCRSVersion returns the OWASP CRS version reported in the most recent
// detection ("" if no events carry one yet). The ruleset is compiled into the
// ward-caddy image, so this reflects what the running edge actually enforces —
// the honest answer to "which rules am I on?" even after an image upgrade.
func (s *Store) LatestCRSVersion(ctx context.Context) string {
	var v string
	err := s.DB.NewSelect().
		TableExpr("waf_events").
		ColumnExpr("crs_version").
		Where("crs_version IS NOT NULL AND crs_version != ''").
		Order("ts DESC").
		Limit(1).
		Scan(ctx, &v)
	if err != nil {
		return ""
	}
	return v
}

// LeanWAFEvent is a minimal projection of a waf_events row for dashboard aggregation.
type LeanWAFEvent struct {
	TS             time.Time `bun:"ts"`
	ServiceID      *string   `bun:"service_id"`
	IsInterrupted  bool      `bun:"is_interrupted"`
	IsAnomalyScore bool      `bun:"is_anomaly_score"`
}

// LeanWAFEventsSince returns minimal event rows at or after `since`, for aggregation.
func (s *Store) LeanWAFEventsSince(ctx context.Context, since time.Time) ([]LeanWAFEvent, error) {
	var rows []LeanWAFEvent
	err := s.DB.NewSelect().
		TableExpr("waf_events").
		ColumnExpr("ts").
		ColumnExpr("service_id").
		ColumnExpr("is_interrupted").
		ColumnExpr("is_anomaly_score").
		Where("ts >= ?", since).
		Scan(ctx, &rows)
	return rows, err
}

// GetServiceIDByHost resolves a hostname (primary or extra) to a service id (nil if
// none). The primary is indexed — the common case; only if that misses do we scan
// the extra-hostname lists (services are few).
func (s *Store) GetServiceIDByHost(ctx context.Context, host string) (*string, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	var row serviceRow
	err := s.DB.NewSelect().Model(&row).Column("id").Where("public_hostname = ?", host).Limit(1).Scan(ctx)
	if err == nil {
		id := row.ID
		return &id, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var rows []serviceRow
	if err := s.DB.NewSelect().Model(&rows).Column("id", "extra_hostnames").Scan(ctx); err != nil {
		return nil, err
	}
	for _, r := range rows {
		var extras []string
		if r.ExtraHostnames != "" {
			_ = json.Unmarshal([]byte(r.ExtraHostnames), &extras)
		}
		for _, h := range extras {
			if strings.EqualFold(strings.TrimSpace(h), host) {
				id := r.ID
				return &id, nil
			}
		}
	}
	return nil, nil
}

// TriggerFilter narrows a TopTriggers query.
type TriggerFilter struct {
	ServiceID string
	Since     time.Time
	Limit     int
}

type triggerRow struct {
	ServiceID     *string   `bun:"service_id"`
	Host          string    `bun:"host"`
	Path          string    `bun:"path"`
	RuleID        int       `bun:"rule_id"`
	RuleMsg       string    `bun:"rule_msg"`
	Severity      string    `bun:"severity"`
	MatchedTarget string    `bun:"matched_target"`
	Hits          int       `bun:"hits"`
	DistinctIPs   int       `bun:"distinct_ips"`
	FirstSeen     time.Time `bun:"first_seen"`
	LastSeen      time.Time `bun:"last_seen"`
}

// TopTriggers clusters detections by (service, path, rule, target), ranked by
// hits. Anomaly aggregators (949xxx) are excluded — they aren't tuning targets.
func (s *Store) TopTriggers(ctx context.Context, f TriggerFilter) ([]model.WAFTrigger, error) {
	q := s.DB.NewSelect().
		TableExpr("waf_events").
		ColumnExpr("service_id").
		ColumnExpr("path").
		ColumnExpr("rule_id").
		ColumnExpr("COALESCE(matched_target, '') AS matched_target").
		ColumnExpr("COALESCE(MAX(host), '') AS host").
		ColumnExpr("COALESCE(MAX(rule_msg), '') AS rule_msg").
		ColumnExpr("COALESCE(MAX(severity), '') AS severity").
		ColumnExpr("COUNT(*) AS hits").
		ColumnExpr("COUNT(DISTINCT client_ip) AS distinct_ips").
		ColumnExpr("MIN(ts) AS first_seen").
		ColumnExpr("MAX(ts) AS last_seen").
		Where("is_anomaly_score = ?", false).
		GroupExpr("service_id, path, rule_id, matched_target").
		OrderExpr("hits DESC")

	if f.ServiceID != "" {
		q = q.Where("service_id = ?", f.ServiceID)
	}
	if !f.Since.IsZero() {
		q = q.Where("ts >= ?", f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	q = q.Limit(limit)

	var rows []triggerRow
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]model.WAFTrigger, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.WAFTrigger{
			ServiceID: r.ServiceID, Host: r.Host, Path: r.Path, RuleID: r.RuleID,
			RuleMsg: r.RuleMsg, Severity: r.Severity, MatchedTarget: r.MatchedTarget,
			Hits: r.Hits, DistinctIPs: r.DistinctIPs, FirstSeen: r.FirstSeen, LastSeen: r.LastSeen,
		})
	}
	return out, nil
}
