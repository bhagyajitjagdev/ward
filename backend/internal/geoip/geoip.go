// Package geoip manages the GeoIP database file that the edge's geolocation
// matcher reads. The file lives in a directory shared between Ward and Caddy
// (WARD_GEOIP_DIR, default /geoip); the "source" is just how it gets there —
// dropped in (mount), uploaded, or downloaded from DB-IP Lite / MaxMind.
package geoip

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// managedName is where Ward writes DBs it fetches or receives. A drop-in file
// under any other *.mmdb name is still detected (see Active).
const managedName = "country.mmdb"

// Dir returns the shared GeoIP directory (WARD_GEOIP_DIR, default /geoip).
func Dir() string {
	if v := os.Getenv("WARD_GEOIP_DIR"); v != "" {
		return v
	}
	return "/geoip"
}

// DB describes the active GeoIP database file.
type DB struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"-"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Active returns the GeoIP database in use: the Ward-managed file if present,
// otherwise the first *.mmdb found (a drop-in). ok is false if none exists.
func Active(dir string) (DB, bool) {
	if fi, err := os.Stat(filepath.Join(dir, managedName)); err == nil && !fi.IsDir() {
		return DB{Filename: managedName, Path: filepath.Join(dir, managedName), Size: fi.Size(), UpdatedAt: fi.ModTime()}, true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return DB{}, false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".mmdb") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			return DB{Filename: e.Name(), Path: filepath.Join(dir, e.Name()), Size: fi.Size(), UpdatedAt: fi.ModTime()}, true
		}
	}
	return DB{}, false
}

// ActivePath returns the active DB path for config generation ("" if none).
func ActivePath(dir string) string {
	if db, ok := Active(dir); ok {
		return db.Path
	}
	return ""
}

// Save writes an uploaded database to the managed path (atomic).
func Save(dir string, r io.Reader) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, managedName), r)
}

// Remove deletes the Ward-managed database (a no-op if absent).
func Remove(dir string) error {
	if err := os.Remove(filepath.Join(dir, managedName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DownloadDBIP fetches the latest DB-IP Lite country database (free, redistributable).
func DownloadDBIP(dir string) error {
	now := time.Now().UTC()
	var lastErr error
	for i := 0; i < 4; i++ { // DB-IP publishes monthly; fall back if this month isn't up yet
		m := now.AddDate(0, -i, 0)
		url := fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%04d-%02d.mmdb.gz", m.Year(), int(m.Month()))
		if err := downloadGz(dir, url); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("db-ip download failed: %w", lastErr)
}

// DownloadMaxMind fetches GeoLite2-Country using the operator's license key.
func DownloadMaxMind(dir, licenseKey string) error {
	if strings.TrimSpace(licenseKey) == "" {
		return errors.New("a MaxMind license key is required")
	}
	url := "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&suffix=tar.gz&license_key=" + strings.TrimSpace(licenseKey)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("maxmind download: %s (check the license key)", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if strings.HasSuffix(hdr.Name, ".mmdb") {
			return Save(dir, tr)
		}
	}
	return errors.New("no .mmdb found in the MaxMind archive")
}

func downloadGz(dir, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	return Save(dir, gz)
}

func writeAtomic(dst string, r io.Reader) error {
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
