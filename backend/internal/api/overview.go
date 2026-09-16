package api

import (
	"net/http"
	"time"
)

// overviewResponse is the dashboard rollup — WAF detections + request volume
// (access_events) + services + blocklist.
type overviewResponse struct {
	Services      int                 `json:"services"`
	WAFServices   int                 `json:"waf_services"`
	Detections24h int                 `json:"detections_24h"`
	Blocked24h    int                 `json:"blocked_24h"`
	Requests24h   int                 `json:"requests_24h"`
	ActiveBlocks  int                 `json:"active_blocks"`
	Activity      []activityBucket    `json:"activity"`
	ByService     []serviceDetections `json:"by_service"`
}

type activityBucket struct {
	Hour       time.Time `json:"hour"`
	Detections int       `json:"detections"`
	Blocked    int       `json:"blocked"`
	Requests   int       `json:"requests"`
}

type serviceDetections struct {
	ServiceID  string `json:"service_id"`
	Detections int    `json:"detections_24h"`
}

// overview aggregates the last 24h in SQL (hourly buckets + per-service counts) —
// never the raw rows in memory, so it stays flat as the access log grows.
func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// 24 hourly buckets ending at the current hour.
	base := time.Now().UTC().Truncate(time.Hour).Add(-23 * time.Hour)

	wafSeries, err := h.store.WAFSeriesSince(ctx, base, 3600)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	byService, err := h.store.WAFCountByServiceSince(ctx, base)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	accessSeries, err := h.store.AccessSeriesSince(ctx, base, "", 3600)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	svcs, err := h.store.ListServices(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	blocks, err := h.store.ListActiveBlocks(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	resp := overviewResponse{ActiveBlocks: len(blocks)}
	for _, s := range svcs {
		if s.Enabled {
			resp.Services++
			if s.WAFEnabled {
				resp.WAFServices++
			}
		}
	}

	buckets := make([]activityBucket, 24)
	for i := range buckets {
		buckets[i] = activityBucket{Hour: base.Add(time.Duration(i) * time.Hour)}
	}
	idx := func(bucketUnix int64) int { return int((bucketUnix - base.Unix()) / 3600) }
	for _, b := range wafSeries {
		resp.Detections24h += int(b.Detections)
		resp.Blocked24h += int(b.Blocked)
		if i := idx(b.Bucket); i >= 0 && i < 24 {
			buckets[i].Detections += int(b.Detections)
			buckets[i].Blocked += int(b.Blocked)
		}
	}
	for _, b := range accessSeries {
		resp.Requests24h += int(b.Requests)
		if i := idx(b.Bucket); i >= 0 && i < 24 {
			buckets[i].Requests += int(b.Requests)
		}
	}
	resp.Activity = buckets
	resp.ByService = make([]serviceDetections, 0, len(byService))
	for _, c := range byService {
		resp.ByService = append(resp.ByService, serviceDetections{ServiceID: c.ServiceID, Detections: int(c.Count)})
	}

	writeJSON(w, http.StatusOK, resp)
}
