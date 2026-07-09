package api

import (
	"encoding/json"
	"net/http"

	"github.com/bhagyajitjagdev/ward/backend/internal/certs"
)

// validTLSMode reports whether m is an accepted service TLS mode.
func validTLSMode(m string) bool {
	switch m {
	case "", "internal", "managed", "none", "custom":
		return true
	}
	return false
}

func (h *Handler) listCertificates(w http.ResponseWriter, r *http.Request) {
	list, err := certs.List(certs.Dir())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) uploadCertificate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Domain  string `json:"domain"`
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if in.Domain == "" || in.CertPEM == "" || in.KeyPEM == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain, cert_pem and key_pem are required"})
		return
	}
	// certs.Save validates the pair + that it covers the domain, then writes to the volume.
	c, err := certs.Save(certs.Dir(), in.Domain, []byte(in.CertPEM), []byte(in.KeyPEM))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// A tls_mode=custom service on this domain now serves the uploaded cert.
	h.reconcile(r.Context())
	h.audit(r, "cert.upload", "cert:"+c.Domain, c.NotAfter.Format("2006-01-02"))
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handler) deleteCertificate(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if _, ok := certs.Get(certs.Dir(), domain); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err := certs.Remove(certs.Dir(), domain); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	h.audit(r, "cert.delete", "cert:"+domain, "")
	h.reconcile(r.Context())
	w.WriteHeader(http.StatusNoContent)
}
