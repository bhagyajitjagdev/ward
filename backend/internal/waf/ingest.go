package waf

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
	"github.com/bhagyajitjagdev/ward/backend/internal/store"
)

// Ingester tails the Coraza JSON audit log (one record per line), parses each
// record, resolves the service by Host, and inserts waf_events. The byte offset
// is persisted in settings so it resumes across restarts.
type Ingester struct {
	store     *store.Store
	path      string
	interval  time.Duration
	hostCache map[string]*string
	mu        sync.Mutex
}

// NewIngester builds an ingester for the given audit-log path.
func NewIngester(s *store.Store, path string) *Ingester {
	return &Ingester{store: s, path: path, interval: 2 * time.Second, hostCache: map[string]*string{}}
}

// SetInterval overrides the poll interval (default 2s).
func (ing *Ingester) SetInterval(d time.Duration) {
	if d > 0 {
		ing.interval = d
	}
}

func (ing *Ingester) offsetKey() string { return "audit_offset:" + ing.path }

// Run tails the audit log until ctx is cancelled.
func (ing *Ingester) Run(ctx context.Context) {
	log.Printf("waf ingester: tailing %s", ing.path)
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
	var events []model.WAFEvent
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			break // EOF or partial line — don't consume an incomplete record
		}
		consumed += int64(len(line))
		evs, perr := ParseAuditRecord(line)
		if perr != nil {
			continue // skip malformed lines
		}
		for i := range evs {
			evs[i].ServiceID = ing.resolveService(ctx, evs[i].Host)
		}
		events = append(events, evs...)
	}

	if len(events) > 0 {
		if err := ing.store.InsertWAFEvents(ctx, events); err != nil {
			log.Printf("waf ingester: insert failed (will retry): %v", err)
			return // don't advance the offset on failure
		}
	}
	if consumed > 0 {
		ing.saveOffset(ctx, offset+consumed)
	}
}

// resolveService maps a host to a service id; only successful hits are cached so
// a service created later is still picked up.
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
