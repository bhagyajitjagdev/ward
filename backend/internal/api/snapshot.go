package api

import (
	"errors"
	"net/http"

	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

func (h *Handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	snaps, err := h.store.ListSnapshots(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

// getSnapshot returns one snapshot including its stored Caddy JSON (omitted from the
// list) — so a snapshot can be fetched and diffed remotely.
func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := h.store.GetSnapshot(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if snap == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "snapshot not found"})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	if h.applier == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no edge configured"})
		return
	}
	id := r.PathValue("id")
	if err := h.applier.Rollback(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "snapshot not found"})
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.audit(r, "config.rollback", "snapshot:"+id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "rolled back", "snapshot_id": id})
}
