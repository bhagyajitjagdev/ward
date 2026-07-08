package api

import (
	"net/http"
	"time"
)

// overviewResponse is the dashboard rollup. It's computed from the WAF-detection
// stream (waf_events) + services + blocklist — Ward has no access-log pipeline yet,
// so there is no total-request count, only detections.
type overviewResponse struct {
	Services      int                 `json:"services"`
	WAFServices   int                 `json:"waf_services"`
	Detections24h int                 `json:"detections_24h"`
	Blocked24h    int                 `json:"blocked_24h"`
	ActiveBlocks  int                 `json:"active_blocks"`
	Activity      []activityBucket    `json:"activity"`
	ByService     []serviceDetections `json:"by_service"`
}

type activityBucket struct {
	Hour       time.Time `json:"hour"`
	Detections int       `json:"detections"`
	Blocked    int       `json:"blocked"`
}

type serviceDetections struct {
	ServiceID  string `json:"service_id"`
	Detections int    `json:"detections_24h"`
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// 24 hourly buckets ending at the current hour.
	base := time.Now().Truncate(time.Hour).Add(-23 * time.Hour)

	events, err := h.store.LeanWAFEventsSince(ctx, base)
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
	byService := map[string]int{}
	for _, e := range events {
		if e.IsAnomalyScore {
			continue // count real detections, not the 949xxx anomaly aggregators
		}
		resp.Detections24h++
		if e.IsInterrupted {
			resp.Blocked24h++
		}
		if e.ServiceID != nil {
			byService[*e.ServiceID]++
		}
		idx := int(e.TS.Truncate(time.Hour).Sub(base) / time.Hour)
		if idx >= 0 && idx < 24 {
			buckets[idx].Detections++
			if e.IsInterrupted {
				buckets[idx].Blocked++
			}
		}
	}
	resp.Activity = buckets
	resp.ByService = make([]serviceDetections, 0, len(byService))
	for id, n := range byService {
		resp.ByService = append(resp.ByService, serviceDetections{ServiceID: id, Detections: n})
	}

	writeJSON(w, http.StatusOK, resp)
}
