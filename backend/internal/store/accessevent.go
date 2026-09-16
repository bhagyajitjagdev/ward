package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

type accessEventRow struct {
	bun.BaseModel `bun:"table:access_events,alias:ae"`

	ID         string    `bun:"id,pk"`
	TS         time.Time `bun:"ts,notnull"`
	ServiceID  *string   `bun:"service_id"`
	Host       string    `bun:"host,notnull"`
	ClientIP   string    `bun:"client_ip,notnull"`
	Method     string    `bun:"method,notnull"`
	Path       string    `bun:"path,notnull"`
	Query      string    `bun:"query,notnull"`
	Status     int       `bun:"status,notnull"`
	DurationMs float64   `bun:"duration_ms,notnull"`
	Bytes      int64     `bun:"bytes,notnull"`
	UserAgent  string    `bun:"user_agent,notnull"`
}

func (r accessEventRow) toModel() model.AccessEvent {
	return model.AccessEvent{
		ID: r.ID, TS: r.TS, ServiceID: r.ServiceID, Host: r.Host, ClientIP: r.ClientIP,
		Method: r.Method, Path: r.Path, Query: r.Query, Status: r.Status,
		DurationMs: r.DurationMs, Bytes: r.Bytes, UserAgent: r.UserAgent,
	}
}

// InsertAccessEvents batch-inserts access events (server-assigned ids).
func (s *Store) InsertAccessEvents(ctx context.Context, events []model.AccessEvent) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]accessEventRow, 0, len(events))
	for _, e := range events {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		rows = append(rows, accessEventRow{
			ID: id.String(), TS: e.TS.UTC(), ServiceID: e.ServiceID, Host: e.Host, ClientIP: e.ClientIP,
			Method: e.Method, Path: e.Path, Query: e.Query, Status: e.Status,
			DurationMs: e.DurationMs, Bytes: e.Bytes, UserAgent: e.UserAgent,
		})
	}
	_, err := s.DB.NewInsert().Model(&rows).Exec(ctx)
	return err
}

// AccessFilter narrows ListAccessEvents.
type AccessFilter struct {
	ServiceID string
	ClientIP  string
	Method    string
	Path      string // prefix match
	Status    int
	Since     time.Time
	Limit     int
}

// ListAccessEvents returns matching events, newest first.
func (s *Store) ListAccessEvents(ctx context.Context, f AccessFilter) ([]model.AccessEvent, error) {
	var rows []accessEventRow
	q := s.DB.NewSelect().Model(&rows).Order("ts DESC")
	if f.ServiceID != "" {
		q = q.Where("service_id = ?", f.ServiceID)
	}
	if f.ClientIP != "" {
		q = q.Where("client_ip = ?", f.ClientIP)
	}
	if f.Method != "" {
		q = q.Where("method = ?", f.Method)
	}
	if f.Path != "" {
		q = q.Where("path LIKE ?", f.Path+"%")
	}
	if f.Status != 0 {
		q = q.Where("status = ?", f.Status)
	}
	if !f.Since.IsZero() {
		q = q.Where("ts > ?", f.Since)
	}
	limit := f.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	if err := q.Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]model.AccessEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.toModel())
	}
	return out, nil
}

// AccessSummary is the whole-window rollup the stats endpoint serves, computed
// in SQL so the raw window is never pulled into Go memory (architecture §6: the
// dashboards must not scan raw rows client-side).
type AccessSummary struct {
	Total int64   `bun:"total"`
	Bytes int64   `bun:"bytes"`
	AvgMs float64 `bun:"avg_ms"`
	S2xx  int64   `bun:"s2xx"`
	S3xx  int64   `bun:"s3xx"`
	S4xx  int64   `bun:"s4xx"`
	S5xx  int64   `bun:"s5xx"`
}

// AccessSummarySince aggregates the events since `since` (optionally one service).
func (s *Store) AccessSummarySince(ctx context.Context, since time.Time, serviceID string) (AccessSummary, error) {
	var out AccessSummary
	q := s.DB.NewSelect().TableExpr("access_events").
		ColumnExpr("COUNT(*) AS total").
		ColumnExpr("CAST(COALESCE(SUM(bytes), 0) AS BIGINT) AS bytes").
		ColumnExpr("CAST(COALESCE(AVG(duration_ms), 0) AS FLOAT) AS avg_ms").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END), 0) AS BIGINT) AS s2xx").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END), 0) AS BIGINT) AS s3xx").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END), 0) AS BIGINT) AS s4xx").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END), 0) AS BIGINT) AS s5xx").
		Where("ts > ?", since)
	if serviceID != "" {
		q = q.Where("service_id = ?", serviceID)
	}
	err := q.Scan(ctx, &out)
	return out, err
}

// AccessP95Since returns the 95th-percentile request duration (ms) over the window,
// via an ordered LIMIT/OFFSET seek — portable and never materialises the window.
// total is the window's row count (from AccessSummarySince).
func (s *Store) AccessP95Since(ctx context.Context, since time.Time, serviceID string, total int64) (float64, error) {
	if total <= 0 {
		return 0, nil
	}
	offset := int(0.95 * float64(total))
	if offset >= int(total) {
		offset = int(total) - 1
	}
	var v float64
	q := s.DB.NewSelect().TableExpr("access_events").ColumnExpr("duration_ms").
		Where("ts > ?", since).OrderExpr("duration_ms ASC").Limit(1).Offset(offset)
	if serviceID != "" {
		q = q.Where("service_id = ?", serviceID)
	}
	err := q.Scan(ctx, &v)
	return v, err
}

// AccessBucket is one time-series point: requests + 5xx in a bucket starting at
// Bucket (Unix seconds).
type AccessBucket struct {
	Bucket   int64 `bun:"bucket"`
	Requests int64 `bun:"requests"`
	Errors   int64 `bun:"errors"`
}

// AccessSeriesSince buckets the window into bucketSec-wide points (GROUP BY in
// SQL; empty buckets are simply absent). Oldest first.
func (s *Store) AccessSeriesSince(ctx context.Context, since time.Time, serviceID string, bucketSec int64) ([]AccessBucket, error) {
	if bucketSec <= 0 {
		bucketSec = 60
	}
	bucket := s.bucketExpr("ts", bucketSec)
	var rows []AccessBucket
	q := s.DB.NewSelect().TableExpr("access_events").
		ColumnExpr(bucket+" AS bucket").
		ColumnExpr("COUNT(*) AS requests").
		ColumnExpr("CAST(COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END), 0) AS BIGINT) AS errors").
		Where("ts > ?", since).
		GroupExpr(bucket).
		OrderExpr("bucket ASC")
	if serviceID != "" {
		q = q.Where("service_id = ?", serviceID)
	}
	err := q.Scan(ctx, &rows)
	return rows, err
}

// AccessPathCount is a request count for one path.
type AccessPathCount struct {
	Path  string `bun:"path" json:"path"`
	Count int64  `bun:"c" json:"count"`
}

// TopAccessPaths returns the busiest paths since `since` (optionally one service).
func (s *Store) TopAccessPaths(ctx context.Context, since time.Time, serviceID string, limit int) ([]AccessPathCount, error) {
	var rows []AccessPathCount
	q := s.DB.NewSelect().Table("access_events").
		ColumnExpr("path").ColumnExpr("count(*) AS c").
		Where("ts > ?", since).GroupExpr("path").OrderExpr("c DESC").Limit(limit)
	if serviceID != "" {
		q = q.Where("service_id = ?", serviceID)
	}
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// CountAccessSince returns the number of access events since `since`.
func (s *Store) CountAccessSince(ctx context.Context, since time.Time) (int64, error) {
	n, err := s.DB.NewSelect().Table("access_events").Where("ts > ?", since).Count(ctx)
	return int64(n), err
}

// PruneAccessEvents deletes events older than `before`; returns rows removed.
func (s *Store) PruneAccessEvents(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.DB.NewDelete().Model((*accessEventRow)(nil)).Where("ts < ?", before).Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
