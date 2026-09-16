package api

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

func (h *Handler) listAccessEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.AccessFilter{
		ServiceID: q.Get("service_id"),
		ClientIP:  q.Get("client_ip"),
		Method:    q.Get("method"),
		Path:      q.Get("path"),
	}
	if v := q.Get("status"); v != "" {
		f.Status, _ = strconv.Atoi(v)
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.Since = t
		}
	}
	events, err := h.store.ListAccessEvents(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

type accessBucket struct {
	Bucket   string `json:"bucket"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
}

type accessStatsResp struct {
	Total    int64                   `json:"total"`
	Status   map[string]int64        `json:"status"`
	Bytes    int64                   `json:"bytes"`
	AvgMs    float64                 `json:"avg_ms"`
	P95Ms    float64                 `json:"p95_ms"`
	Series   []accessBucket          `json:"series"`
	TopPaths []store.AccessPathCount `json:"top_paths"`
}

// accessStats serves the window's totals, status mix, latency, a time series, and
// top paths — all aggregated in SQL (a handful of small result sets), never by
// loading the window's rows into Go. The window defaults to the last 24h.
func (h *Handler) accessStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	since := time.Now().UTC().Add(-24 * time.Hour)
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	ctx := r.Context()
	sum, err := h.store.AccessSummarySince(ctx, since, serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	p95, err := h.store.AccessP95Since(ctx, since, serviceID, sum.Total)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	bucketSec := int64(math.Max(60, time.Since(since).Seconds()/120)) // ~120 points across the window
	series, err := h.store.AccessSeriesSince(ctx, since, serviceID, bucketSec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	resp := accessStatsResp{
		Total:    sum.Total,
		Bytes:    sum.Bytes,
		AvgMs:    sum.AvgMs,
		P95Ms:    p95,
		Status:   map[string]int64{"2xx": sum.S2xx, "3xx": sum.S3xx, "4xx": sum.S4xx, "5xx": sum.S5xx},
		Series:   make([]accessBucket, 0, len(series)),
		TopPaths: []store.AccessPathCount{},
	}
	for _, b := range series {
		resp.Series = append(resp.Series, accessBucket{
			Bucket:   time.Unix(b.Bucket, 0).UTC().Format(time.RFC3339),
			Requests: b.Requests,
			Errors:   b.Errors,
		})
	}
	if tp, err := h.store.TopAccessPaths(ctx, since, serviceID, 10); err == nil && tp != nil {
		resp.TopPaths = tp // keep the [] init on empty — a nil slice marshals to JSON null and crashes the UI
	}
	writeJSON(w, http.StatusOK, resp)
}
