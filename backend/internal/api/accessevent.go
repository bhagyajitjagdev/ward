package api

import (
	"math"
	"net/http"
	"sort"
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

// accessStats aggregates the bounded window in Go (totals, status mix, latency,
// a time series, and top paths). The window defaults to the last 24h.
func (h *Handler) accessStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	serviceID := q.Get("service_id")
	since := time.Now().UTC().Add(-24 * time.Hour)
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	events, err := h.store.LeanAccessSince(r.Context(), since, serviceID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	resp := accessStatsResp{
		Status:   map[string]int64{"2xx": 0, "3xx": 0, "4xx": 0, "5xx": 0},
		Series:   []accessBucket{},
		TopPaths: []store.AccessPathCount{},
	}
	bucketSec := int64(math.Max(60, time.Since(since).Seconds()/120)) // ~120 points across the window
	buckets := map[int64]*accessBucket{}
	durs := make([]float64, 0, len(events))
	var durSum float64
	for _, e := range events {
		resp.Total++
		resp.Bytes += e.Bytes
		switch {
		case e.Status >= 500:
			resp.Status["5xx"]++
		case e.Status >= 400:
			resp.Status["4xx"]++
		case e.Status >= 300:
			resp.Status["3xx"]++
		case e.Status >= 200:
			resp.Status["2xx"]++
		}
		durSum += e.DurationMs
		durs = append(durs, e.DurationMs)
		b := (e.TS.Unix() / bucketSec) * bucketSec
		bk := buckets[b]
		if bk == nil {
			bk = &accessBucket{Bucket: time.Unix(b, 0).UTC().Format(time.RFC3339)}
			buckets[b] = bk
		}
		bk.Requests++
		if e.Status >= 500 {
			bk.Errors++
		}
	}
	if resp.Total > 0 {
		resp.AvgMs = durSum / float64(resp.Total)
		sort.Float64s(durs)
		idx := int(0.95 * float64(len(durs)))
		if idx >= len(durs) {
			idx = len(durs) - 1
		}
		resp.P95Ms = durs[idx]
	}
	keys := make([]int64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		resp.Series = append(resp.Series, *buckets[k])
	}
	if tp, err := h.store.TopAccessPaths(r.Context(), since, serviceID, 10); err == nil && tp != nil {
		resp.TopPaths = tp // keep the [] init on empty — a nil slice marshals to JSON null and crashes the UI
	}
	writeJSON(w, http.StatusOK, resp)
}
