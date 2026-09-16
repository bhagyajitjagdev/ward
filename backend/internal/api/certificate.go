package api

import (
	"encoding/json"
	"net/http"
	"strings"

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
	if in.CertPEM == "" || in.KeyPEM == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cert_pem and key_pem are required"})
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
	resp := struct {
		certs.Cert
		Warning string `json:"warning,omitempty"`
	}{Cert: c}
	if len(c.IPSANs) > 0 {
		resp.Warning = "certificate carries IP address SANs (" + strings.Join(c.IPSANs, ", ") +
			"). Ward enforces strict SNI, so clients that connect by IP send no SNI and are refused — reach this service by hostname."
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) deleteCertificate(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	c, ok := certs.Get(certs.Dir(), domain)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// Guard: a tls_mode=custom service whose hostname only this cert secures would
	// silently fall out of skip_certificates on the next reconcile and have Caddy try
	// to auto-issue for it. Refuse until the service is moved or a replacement exists.
	svcs, err := h.store.ListServices(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var inUse []string
	for _, s := range svcs {
		if !s.Enabled || s.TLSMode != "custom" {
			continue
		}
		for _, hn := range s.PublicHostnames {
			if c.Secures(hn) && !certs.CoversExcept(certs.Dir(), hn, c.Domain) {
				inUse = append(inUse, s.Name+" ("+hn+")")
				break
			}
		}
	}
	if len(inUse) > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "certificate is still used by " +
			strings.Join(inUse, ", ") + " — switch those services to another TLS mode or upload a replacement first"})
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
