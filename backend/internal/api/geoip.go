package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/geoip"
)

type geoipStatus struct {
	Present   bool       `json:"present"`
	Source    string     `json:"source,omitempty"`
	Filename  string     `json:"filename,omitempty"`
	Size      int64      `json:"size,omitempty"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	Dir       string     `json:"dir"`
}

func (h *Handler) geoipStatusVal(ctx context.Context) geoipStatus {
	dir := geoip.Dir()
	src, _ := h.store.GetSetting(ctx, "geoip.source")
	st := geoipStatus{Source: src, Dir: dir}
	if db, ok := geoip.Active(dir); ok {
		st.Present = true
		st.Filename = db.Filename
		st.Size = db.Size
		t := db.UpdatedAt
		st.UpdatedAt = &t
		if src == "" {
			st.Source = "mount" // present but no recorded source → dropped into the folder
		}
	}
	return st
}

func (h *Handler) geoipGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.geoipStatusVal(r.Context()))
}

// applyGeoipChange records the source, regenerates the edge config (so geo rules
// activate now that a DB exists), and audits.
func (h *Handler) applyGeoipChange(r *http.Request, source string) {
	_ = h.store.SetSetting(r.Context(), "geoip.source", source)
	h.reconcile(r.Context())
	h.audit(r, "geoip.update", "geoip:"+source, "")
}

func (h *Handler) geoipDBIP(w http.ResponseWriter, r *http.Request) {
	if err := geoip.DownloadDBIP(geoip.Dir()); err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	h.applyGeoipChange(r, "dbip")
	writeJSON(w, http.StatusOK, h.geoipStatusVal(r.Context()))
}

func (h *Handler) geoipMaxMind(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LicenseKey string `json:"license_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if err := geoip.DownloadMaxMind(geoip.Dir(), in.LicenseKey); err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	_ = h.store.SetSetting(r.Context(), "geoip.maxmind_key", in.LicenseKey)
	h.applyGeoipChange(r, "maxmind")
	writeJSON(w, http.StatusOK, h.geoipStatusVal(r.Context()))
}

func (h *Handler) geoipUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil { // up to 128 MiB
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected a multipart upload"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a .mmdb file is required (field 'file')"})
		return
	}
	defer file.Close()
	if err := geoip.Save(geoip.Dir(), file); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.applyGeoipChange(r, "upload")
	writeJSON(w, http.StatusOK, h.geoipStatusVal(r.Context()))
}

func (h *Handler) geoipDelete(w http.ResponseWriter, r *http.Request) {
	if err := geoip.Remove(geoip.Dir()); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.SetSetting(r.Context(), "geoip.source", "")
	h.reconcile(r.Context())
	h.audit(r, "geoip.delete", "geoip", "")
	w.WriteHeader(http.StatusNoContent)
}
