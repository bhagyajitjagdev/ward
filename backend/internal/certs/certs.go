// Package certs manages bring-your-own TLS certificates on a shared volume that
// both Ward (writer) and Caddy (reader, via load_files) mount. One folder per
// domain: <dir>/custom/<domain>/{cert.pem,key.pem}. No DB — the volume is the
// store; the generated Caddy config (snapshotted for rollback) holds the paths.
// (Let's Encrypt + internal-CA certs are managed by Caddy in its own data volume.)
package certs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	certFile = "cert.pem"
	keyFile  = "key.pem"
)

// Dir returns the certs volume (WARD_CERTS_DIR, default /certs).
func Dir() string {
	if v := os.Getenv("WARD_CERTS_DIR"); v != "" {
		return v
	}
	return "/certs"
}

func customRoot(dir string) string { return filepath.Join(dir, "custom") }

// Cert describes one uploaded certificate on the volume. The PEM paths are for
// config-gen only (load_files); they never leave the process (json:"-").
type Cert struct {
	Domain    string    `json:"domain"`     // folder name = the host it secures
	Subjects  []string  `json:"subjects"`   // CN + SANs parsed from the cert
	NotAfter  time.Time `json:"not_after"`  // expiry
	UpdatedAt time.Time `json:"updated_at"` // cert file mtime
	CertPath  string    `json:"-"`
	KeyPath   string    `json:"-"`
}

// safeDomain validates a domain for use as a folder name (no path traversal).
func safeDomain(d string) (string, error) {
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" {
		return "", fmt.Errorf("domain is required")
	}
	if len(d) > 253 || strings.ContainsAny(d, `/\`) || strings.Contains(d, "..") {
		return "", fmt.Errorf("invalid domain")
	}
	for _, r := range d {
		if !(r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return "", fmt.Errorf("invalid domain (letters, digits, dot, hyphen only)")
		}
	}
	return d, nil
}

// Save validates the cert/key pair and that it covers the domain, then atomically
// writes the PEMs to <dir>/custom/<domain>/. Overwrites any existing cert there.
func Save(dir, domain string, certPEM, keyPEM []byte) (Cert, error) {
	d, err := safeDomain(domain)
	if err != nil {
		return Cert{}, err
	}
	leaf, err := validatePair(certPEM, keyPEM)
	if err != nil {
		return Cert{}, err
	}
	if leaf.VerifyHostname(d) != nil {
		return Cert{}, fmt.Errorf("certificate does not cover %s (subjects: %s)", d, strings.Join(subjectsOf(leaf), ", "))
	}
	folder := filepath.Join(customRoot(dir), d)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return Cert{}, err
	}
	if err := writeAtomic(filepath.Join(folder, certFile), certPEM, 0o644); err != nil {
		return Cert{}, err
	}
	if err := writeAtomic(filepath.Join(folder, keyFile), keyPEM, 0o600); err != nil {
		return Cert{}, err
	}
	return certFromLeaf(dir, d, leaf), nil
}

// List returns every uploaded cert (parsed), sorted by domain.
func List(dir string) ([]Cert, error) {
	entries, err := os.ReadDir(customRoot(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return []Cert{}, nil
		}
		return nil, err
	}
	out := []Cert{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if c, ok := Get(dir, e.Name()); ok {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out, nil
}

// Get parses the cert for one domain; ok=false if missing/unreadable.
func Get(dir, domain string) (Cert, bool) {
	d, err := safeDomain(domain)
	if err != nil {
		return Cert{}, false
	}
	raw, err := os.ReadFile(filepath.Join(customRoot(dir), d, certFile))
	if err != nil {
		return Cert{}, false
	}
	leaf, err := parseLeaf(raw)
	if err != nil {
		return Cert{}, false
	}
	return certFromLeaf(dir, d, leaf), true
}

// Remove deletes a domain's cert folder.
func Remove(dir, domain string) error {
	d, err := safeDomain(domain)
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(customRoot(dir), d))
}

func certFromLeaf(dir, domain string, leaf *x509.Certificate) Cert {
	folder := filepath.Join(customRoot(dir), domain)
	c := Cert{
		Domain:   domain,
		Subjects: subjectsOf(leaf),
		NotAfter: leaf.NotAfter,
		CertPath: filepath.Join(folder, certFile),
		KeyPath:  filepath.Join(folder, keyFile),
	}
	if fi, err := os.Stat(c.CertPath); err == nil {
		c.UpdatedAt = fi.ModTime().UTC()
	}
	return c
}

func subjectsOf(leaf *x509.Certificate) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(leaf.Subject.CommonName)
	for _, n := range leaf.DNSNames {
		add(n)
	}
	return out
}

// SANMatches reports whether a certificate subject (CN or SAN entry) secures host,
// honoring a single-label wildcard (e.g. *.example.com secures a.example.com but not
// b.a.example.com).
func SANMatches(host, san string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	san = strings.ToLower(strings.TrimSpace(san))
	if host == "" || san == "" {
		return false
	}
	if san == host {
		return true
	}
	if strings.HasPrefix(san, "*.") {
		suffix := san[1:] // ".example.com"
		if strings.HasSuffix(host, suffix) {
			label := host[:len(host)-len(suffix)]
			return label != "" && !strings.Contains(label, ".")
		}
	}
	return false
}

// Covers reports whether any uploaded cert secures host (by CN/SAN, incl. wildcards).
func Covers(dir, host string) bool {
	list, err := List(dir)
	if err != nil {
		return false
	}
	for _, c := range list {
		for _, s := range c.Subjects {
			if SANMatches(host, s) {
				return true
			}
		}
	}
	return false
}

func validatePair(certPEM, keyPEM []byte) (*x509.Certificate, error) {
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, fmt.Errorf("cert and key don't match or aren't valid PEM")
	}
	return parseLeaf(certPEM)
}

func parseLeaf(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM certificate found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func writeAtomic(dst string, data []byte, perm os.FileMode) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
