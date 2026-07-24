// Command appserver is the e2e upstream — a tiny backend Ward proxies to. It echoes
// requests and serves SSE + WebSocket so the harness can exercise streaming, WAF
// payloads, header rewriting, path stripping, and basic-auth end to end.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func main() {
	mux := http.NewServeMux()

	// Root + arbitrary paths: reflect the path so proxy/strip-prefix tests can assert.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"app": "wardtest-upstream", "path": r.URL.Path})
	})

	// Echoes ?q= — the target for WAF payload tests (a SQLi in q is what CRS trips on).
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "you sent: %s\n", r.URL.Query().Get("q"))
	})

	// A large, compressible body — so the compression test can see Content-Encoding.
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		line := "the quick brown fox jumps over the lazy dog 0123456789\n"
		for i := 0; i < 200; i++ {
			fmt.Fprint(w, line)
		}
	})

	// Reflects a request header back (so "forward header to upstream" is verifiable) and
	// echoes the seen headers.
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Saw-Test", r.Header.Get("X-Test"))
		writeJSON(w, map[string]any{"path": r.URL.Path, "x_test": r.Header.Get("X-Test")})
	})

	// SSE: 8 events, 200ms apart. If a proxy/WAF buffers the response the client sees
	// them all at once at the end; if it streams, they arrive spread out.
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}
		for i := 1; i <= 8; i++ {
			fmt.Fprintf(w, "data: event %d %d\n\n", i, time.Now().UnixMilli())
			fl.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
		fmt.Fprint(w, "data: done\n\n")
		fl.Flush()
	})

	// WebSocket echo — prefixes "echo: " so the harness can assert a round trip.
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			if err := c.Write(ctx, typ, append([]byte("echo: "), data...)); err != nil {
				return
			}
		}
	})

	srv := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Println("appserver listening on :8080")
	log.Fatal(srv.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
