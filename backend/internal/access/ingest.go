// Package access tails Caddy's structured JSON access log into access_events —
// the same log file an external pipeline (Promtail/Vector -> Loki -> Grafana) can
// consume. Mirrors internal/waf's ingester (offset-tracked, host->service mapping).
package access

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// Ingester tails Caddy's JSON access log (one request per line).
type Ingester struct {
	store     *store.Store
	path      string
	interval  time.Duration
	hostCache map[string]*string
	mu        sync.Mutex
}

// NewIngester builds an ingester for the given access-log path.
func NewIngester(s *store.Store, path string) *Ingester {
	return &Ingester{store: s, path: path, interval: 2 * time.Second, hostCache: map[string]*string{}}
}

// SetInterval overrides the poll interval (default 2s).
func (ing *Ingester) SetInterval(d time.Duration) {
	if d > 0 {
		ing.interval = d
	}
}

func (ing *Ingester) offsetKey() string { return "access_offset:" + ing.path }

// Run tails the access log until ctx is cancelled.
func (ing *Ingester) Run(ctx context.Context) {
	log.Printf("access ingester: tailing %s", ing.path)
	t := time.NewTicker(ing.interval)
	defer t.Stop()
	ing.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ing.tick(ctx)
		}
	}
}

// caddyLog is the subset of Caddy's JSON access log we keep.
type caddyLog struct {
	TS      float64 `json:"ts"`
	Msg     string  `json:"msg"`
	Status  int     `json:"status"`
	Size    int64   `json:"size"`
	Dur     float64 `json:"duration"`
	Request struct {
		ClientIP string              `json:"client_ip"`
		RemoteIP string              `json:"remote_ip"`
		Method   string              `json:"method"`
		Host     string              `json:"host"`
		URI      string              `json:"uri"`
		Headers  map[string][]string `json:"headers"`
	} `json:"request"`
}

func parseLine(line []byte) (model.AccessEvent, bool) {
	var l caddyLog
	if err := json.Unmarshal(line, &l); err != nil || l.Msg != "handled request" {
		return model.AccessEvent{}, false
	}
	host := l.Request.Host
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.Contains(host, "]") {
		host = host[:i] // strip :port (leave bracketed IPv6 literals alone)
	}
	path, query := l.Request.URI, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path, query = path[:i], path[i+1:]
	}
	ip := l.Request.ClientIP
	if ip == "" {
		ip = l.Request.RemoteIP
	}
	ua := ""
	if v := l.Request.Headers["User-Agent"]; len(v) > 0 {
		ua = v[0]
	}
	sec := int64(l.TS)
	return model.AccessEvent{
		TS:         time.Unix(sec, int64((l.TS-float64(sec))*1e9)).UTC(),
		Host:       host,
		ClientIP:   ip,
		Method:     l.Request.Method,
		Path:       path,
		Query:      query,
		Status:     l.Status,
		DurationMs: l.Dur * 1000,
		Bytes:      l.Size,
		UserAgent:  ua,
	}, true
}

func (ing *Ingester) tick(ctx context.Context) {
	f, err := os.Open(ing.path)
	if err != nil {
		return // file may not exist yet
	}
	defer f.Close()

	offset := ing.loadOffset(ctx)
	if st, err := f.Stat(); err == nil && st.Size() < offset {
		offset = 0 // rotated / truncated
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return
	}

	r := bufio.NewReader(f)
	var consumed int64
	var events []model.AccessEvent
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break // EOF or partial line — don't consume an incomplete record
		}
		consumed += int64(len(line))
		ev, ok := parseLine(line)
		if !ok {
			continue
		}
		ev.ServiceID = ing.resolveService(ctx, ev.Host)
		events = append(events, ev)
	}

	if len(events) > 0 {
		if err := ing.store.InsertAccessEvents(ctx, events); err != nil {
			log.Printf("access ingester: insert failed (will retry): %v", err)
			return // don't advance the offset on failure
		}
	}
	if consumed > 0 {
		ing.saveOffset(ctx, offset+consumed)
	}
}

// resolveService maps a host to a service id (cached on hit, so a service created
// later is still picked up).
func (ing *Ingester) resolveService(ctx context.Context, host string) *string {
	if host == "" {
		return nil
	}
	ing.mu.Lock()
	if id, ok := ing.hostCache[host]; ok {
		ing.mu.Unlock()
		return id
	}
	ing.mu.Unlock()

	id, err := ing.store.GetServiceIDByHost(ctx, host)
	if err != nil {
		return nil
	}
	if id != nil {
		ing.mu.Lock()
		ing.hostCache[host] = id
		ing.mu.Unlock()
	}
	return id
}

func (ing *Ingester) loadOffset(ctx context.Context) int64 {
	v, _ := ing.store.GetSetting(ctx, ing.offsetKey())
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func (ing *Ingester) saveOffset(ctx context.Context, off int64) {
	_ = ing.store.SetSetting(ctx, ing.offsetKey(), strconv.FormatInt(off, 10))
}
