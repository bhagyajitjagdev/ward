// Command e2e drives Ward's API and asserts behaviour through the real edge
// (Caddy + Coraza), all inside the compose network. It exits non-zero if any check
// fails, so CI can gate on `docker compose up --exit-code-from tester`.
//
// Each check creates its own service(s) with unique hostnames and cleans up, so they
// stay independent. Env: WARD_API, EDGE, UPSTREAM, UPSTREAM2 (in-network defaults).
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

var (
	wardAPI  = env("WARD_API", "http://ward:8080/api")
	edgeURL  = env("EDGE", "http://caddy:80")
	upstream = env("UPSTREAM", "appserver:8080")
	token    string
	failures int
	httpc    = &http.Client{Timeout: 20 * time.Second}
)

func main() {
	waitReady()
	setup()

	check("proxy", checkProxy)
	check("multi-hostname", checkMultiHostname)
	check("waf-enforce", checkWAFEnforce)
	check("waf-detect+crs-id", checkWAFDetectAndCRSID)
	check("waf-exclusion", checkExclusion)
	check("skip-paths-sse", checkSkipSSE)
	check("skip-paths-ws", checkSkipWS)
	check("skip-paths-still-block", checkSkipStillBlocks)
	check("ip-blocklist", checkIPBlock)
	check("rate-limit", checkRateLimit)
	check("http-security-headers", checkSecurityHeaders)
	check("http-add-remove-header", checkAddRemoveHeader)
	check("http-strip-prefix", checkStripPrefix)
	check("http-basic-auth", checkBasicAuth)
	check("http-compression", checkCompression)
	check("raw-caddyfile", checkRawCaddy)
	check("edge-versions", checkEdgeVersions)
	check("snapshots", checkSnapshots)

	if failures > 0 {
		fmt.Printf("\n%d check(s) FAILED\n", failures)
		os.Exit(1)
	}
	fmt.Println("\nall checks passed ✓")
}

// ── checks ────────────────────────────────────────────────────────────────────

func checkProxy() error {
	id, done, err := mkService(svcSpec("proxy.test", false, ""))
	if err != nil {
		return err
	}
	defer done()
	_ = id
	return retry(10, 300*time.Millisecond, func() error {
		st, body := edge("GET", "proxy.test", "/", nil, "")
		if st != 200 || !strings.Contains(body, "wardtest-upstream") {
			return fmt.Errorf("GET / → %d %q", st, trim(body))
		}
		return nil
	})
}

func checkMultiHostname() error {
	spec := svcSpec("mh-a.test", false, "")
	spec["public_hostnames"] = []string{"mh-a.test", "mh-b.test"}
	id, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	_ = id
	for _, h := range []string{"mh-a.test", "mh-b.test"} {
		if st, _ := edge("GET", h, "/", nil, ""); st != 200 {
			return fmt.Errorf("host %s → %d", h, st)
		}
	}
	return nil
}

func checkWAFEnforce() error {
	_, done, err := mkService(svcSpec("wafon.test", true, "On"))
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	if st, _ := edge("GET", "wafon.test", "/query?q="+sqli, nil, ""); st != 403 {
		return fmt.Errorf("SQLi should be 403, got %d", st)
	}
	if st, _ := edge("GET", "wafon.test", "/query?q="+xss, nil, ""); st != 403 {
		return fmt.Errorf("XSS should be 403, got %d", st)
	}
	if st, _ := edge("GET", "wafon.test", "/query?q=hello", nil, ""); st != 200 {
		return fmt.Errorf("benign should be 200, got %d", st)
	}
	return nil
}

func checkWAFDetectAndCRSID() error {
	id, done, err := mkService(svcSpec("wafdet.test", true, "DetectionOnly"))
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	if st, _ := edge("GET", "wafdet.test", "/query?q="+sqli, nil, ""); st != 200 {
		return fmt.Errorf("detection-only should pass (200), got %d", st)
	}
	// The detection must be logged, and rule 942100 (libinjection SQLi) must still exist
	// in the CRS — this is the rule-ID guard against a CRS renumber breaking exclusions.
	return retry(20, 300*time.Millisecond, func() error {
		var evs []map[string]any
		if _, b := api("GET", "/waf-events?service_id="+id+"&limit=50", nil); true {
			_ = json.Unmarshal(b, &evs)
		}
		for _, e := range evs {
			if fmt.Sprint(e["rule_id"]) == "942100" {
				return nil
			}
		}
		return fmt.Errorf("no waf-event with rule_id 942100 yet (%d events)", len(evs))
	})
}

func checkExclusion() error {
	id, done, err := mkService(svcSpec("excl.test", true, "On"))
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	if st, _ := edge("GET", "excl.test", "/query?q="+sqli, nil, ""); st != 403 {
		return fmt.Errorf("precondition: SQLi should be 403, got %d", st)
	}
	// Scoped exclusion: drop the SQLi rule for /query only.
	st, b := api("POST", "/waf-exclusions", map[string]any{
		"rule_id": 942100, "scope": "service", "service_id": id,
		"path": "/query", "path_match": "prefix",
	})
	if st != 201 {
		return fmt.Errorf("create exclusion → %d %s", st, trim(string(b)))
	}
	time.Sleep(500 * time.Millisecond)
	if st, _ := edge("GET", "excl.test", "/query?q="+sqli, nil, ""); st == 403 {
		return fmt.Errorf("SQLi on /query should be silenced after exclusion, still 403")
	}
	if st, _ := edge("GET", "excl.test", "/other?q="+sqli, nil, ""); st != 403 {
		return fmt.Errorf("SQLi on /other should still be 403 (exclusion is path-scoped), got %d", st)
	}
	return nil
}

func checkSkipSSE() error {
	spec := svcSpec("sse.test", true, "On")
	spec["waf_skip_paths"] = []string{"/sse"}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	first, last, n, err := streamSSE("sse.test", "/sse")
	if err != nil {
		return err
	}
	if n < 5 {
		return fmt.Errorf("expected ≥5 SSE events, got %d", n)
	}
	if spread := last.Sub(first); spread < 800*time.Millisecond {
		return fmt.Errorf("SSE arrived buffered (spread %v) — not streaming", spread)
	}
	return nil
}

func checkSkipWS() error {
	// No skip path listed — WebSocket upgrades bypass the WAF automatically.
	_, done, err := mkService(svcSpec("ws.test", true, "On"))
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	reply, err := wsEcho("ws.test", "/ws", "hello")
	if err != nil {
		return err
	}
	if reply != "echo: hello" {
		return fmt.Errorf("ws reply %q", reply)
	}
	return nil
}

func checkSkipStillBlocks() error {
	spec := svcSpec("skipblock.test", true, "On")
	spec["waf_skip_paths"] = []string{"/sse"}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	if st, _ := edge("GET", "skipblock.test", "/query?q="+sqli, nil, ""); st != 403 {
		return fmt.Errorf("WAF should still block a non-skip path, got %d", st)
	}
	return nil
}

func checkIPBlock() error {
	id, done, err := mkService(svcSpec("block.test", false, ""))
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	st, b := api("POST", "/blocklist", map[string]any{
		"cidr": "0.0.0.0/0", "scope": "service", "service_id": id, "mode": "block",
	})
	if st != 201 {
		return fmt.Errorf("create block → %d %s", st, trim(string(b)))
	}
	var blk map[string]any
	_ = json.Unmarshal(b, &blk)
	time.Sleep(400 * time.Millisecond)
	if st, _ := edge("GET", "block.test", "/", nil, ""); st != 403 {
		return fmt.Errorf("blocked request should be 403, got %d", st)
	}
	if bs, _ := api("DELETE", "/blocklist/"+fmt.Sprint(blk["id"]), nil); bs != 204 {
		return fmt.Errorf("delete block → %d", bs)
	}
	return retry(10, 300*time.Millisecond, func() error {
		if st, _ := edge("GET", "block.test", "/", nil, ""); st != 200 {
			return fmt.Errorf("after unblock → %d", st)
		}
		return nil
	})
}

func checkRateLimit() error {
	id, done, err := mkService(svcSpec("rl.test", false, ""))
	if err != nil {
		return err
	}
	defer done()
	if st, b := api("POST", "/rate-limits", map[string]any{
		"scope": "service", "service_id": id, "max_events": 2, "window": "10s",
	}); st != 201 {
		return fmt.Errorf("create rate-limit → %d %s", st, trim(string(b)))
	}
	time.Sleep(400 * time.Millisecond)
	got429 := false
	for i := 0; i < 8; i++ {
		if st, _ := edge("GET", "rl.test", "/", nil, ""); st == 429 {
			got429 = true
			break
		}
	}
	if !got429 {
		return fmt.Errorf("burst of 8 never hit the 2/10s limit (no 429)")
	}
	return nil
}

func checkSecurityHeaders() error {
	spec := svcSpec("sec.test", false, "")
	spec["http"] = map[string]any{"security_headers": true}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	return retry(10, 300*time.Millisecond, func() error {
		resp, _, err := edgeResp("GET", "sec.test", "/", nil)
		if err != nil {
			return err
		}
		if resp.Header.Get("X-Frame-Options") == "" {
			return fmt.Errorf("missing X-Frame-Options")
		}
		return nil
	})
}

func checkAddRemoveHeader() error {
	spec := svcSpec("hdr.test", false, "")
	spec["http"] = map[string]any{
		"response_headers": map[string]string{"X-Foo": "bar"},
		"remove_headers":   []string{"X-Saw-Test"},
	}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	return retry(10, 300*time.Millisecond, func() error {
		resp, _, err := edgeResp("GET", "hdr.test", "/echo", map[string]string{"X-Test": "v"})
		if err != nil {
			return err
		}
		if resp.Header.Get("X-Foo") != "bar" {
			return fmt.Errorf("added header X-Foo missing")
		}
		if resp.Header.Get("X-Saw-Test") != "" {
			return fmt.Errorf("X-Saw-Test should have been stripped")
		}
		return nil
	})
}

func checkStripPrefix() error {
	spec := svcSpec("strip.test", false, "")
	spec["http"] = map[string]any{"strip_path_prefix": "/api"}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	return retry(10, 300*time.Millisecond, func() error {
		st, body := edge("GET", "strip.test", "/api/echo", nil, "")
		if st != 200 || !strings.Contains(body, `"path":"/echo"`) {
			return fmt.Errorf("strip-prefix: upstream should see /echo, got %d %q", st, trim(body))
		}
		return nil
	})
}

func checkBasicAuth() error {
	spec := svcSpec("auth.test", false, "")
	spec["http"] = map[string]any{"basic_auth_user": "u", "basic_auth_password": "p"}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	time.Sleep(300 * time.Millisecond)
	if st, _ := edge("GET", "auth.test", "/", nil, ""); st != 401 {
		return fmt.Errorf("no creds should be 401, got %d", st)
	}
	if st, _ := edge("GET", "auth.test", "/", map[string]string{"Authorization": basic("u", "p")}, ""); st != 200 {
		return fmt.Errorf("with creds should be 200, got %d", st)
	}
	return nil
}

func checkCompression() error {
	spec := svcSpec("gz.test", false, "")
	spec["http"] = map[string]any{"compression": true}
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	return retry(10, 300*time.Millisecond, func() error {
		resp, _, err := edgeResp("GET", "gz.test", "/big", map[string]string{"Accept-Encoding": "gzip"})
		if err != nil {
			return err
		}
		if resp.Header.Get("Content-Encoding") != "gzip" {
			return fmt.Errorf("expected gzip, got %q", resp.Header.Get("Content-Encoding"))
		}
		return nil
	})
}

func checkRawCaddy() error {
	spec := svcSpec("raw.test", false, "")
	spec["raw_caddy"] = "redir /old /new 302"
	_, done, err := mkService(spec)
	if err != nil {
		return err
	}
	defer done()
	if err := retry(10, 300*time.Millisecond, func() error {
		resp, _, err := edgeResp("GET", "raw.test", "/old", nil)
		if err != nil {
			return err
		}
		if resp.StatusCode != 302 || !strings.Contains(resp.Header.Get("Location"), "/new") {
			return fmt.Errorf("redir → %d loc=%q", resp.StatusCode, resp.Header.Get("Location"))
		}
		return nil
	}); err != nil {
		return err
	}
	// An invalid fragment must be rejected at save (validate-before-apply).
	bad := svcSpec("rawbad.test", false, "")
	bad["raw_caddy"] = "this is not valid caddyfile {{{"
	if st, _ := api("POST", "/services", bad); st == 201 {
		return fmt.Errorf("invalid raw_caddy should have been rejected, got 201")
	}
	return nil
}

func checkEdgeVersions() error {
	_, b := api("GET", "/settings", nil)
	var s struct {
		EdgeVersions map[string]string `json:"edge_versions"`
	}
	_ = json.Unmarshal(b, &s)
	if s.EdgeVersions["caddy"] == "" {
		return fmt.Errorf("settings.edge_versions.caddy is empty")
	}
	return nil
}

func checkSnapshots() error {
	// Services were created/deleted above, so snapshots must exist.
	_, b := api("GET", "/config-snapshots", nil)
	var snaps []map[string]any
	_ = json.Unmarshal(b, &snaps)
	if len(snaps) == 0 {
		return fmt.Errorf("expected config snapshots to exist")
	}
	return nil
}

// ── payload + edge/ws/sse helpers ───────────────────────────────────────────────

const (
	sqli = "1%27%20OR%20%271%27%3D%271" // 1' OR '1'='1
	xss  = "%3Cscript%3Ealert(1)%3C/script%3E"
)

func svcSpec(host string, waf bool, mode string) map[string]any {
	return map[string]any{
		"name":             host,
		"public_hostnames": []string{host},
		"upstreams":        []string{upstream},
		"tls_mode":         "none",
		"waf_enabled":      waf,
		"waf_mode":         mode,
		"enabled":          true,
	}
}

// mkService creates a service and returns its id + a cleanup func.
func mkService(spec map[string]any) (string, func(), error) {
	st, b := api("POST", "/services", spec)
	if st != 201 {
		return "", func() {}, fmt.Errorf("create service → %d %s", st, trim(string(b)))
	}
	var svc map[string]any
	if err := json.Unmarshal(b, &svc); err != nil {
		return "", func() {}, err
	}
	id := fmt.Sprint(svc["id"])
	return id, func() { api("DELETE", "/services/"+id, nil) }, nil
}

// edge sends a request through Caddy with the given Host, returning status + body.
func edge(method, host, path string, headers map[string]string, _ string) (int, string) {
	resp, body, err := edgeResp(method, host, path, headers)
	if err != nil {
		return 0, err.Error()
	}
	return resp.StatusCode, body
}

func edgeResp(method, host, path string, headers map[string]string) (*http.Response, string, error) {
	req, _ := http.NewRequest(method, edgeURL+path, nil)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Don't auto-follow redirects (we assert on 302) and don't auto-decompress.
	c := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}, Transport: &http.Transport{DisableCompression: true}}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b), nil
}

// streamSSE reads the SSE stream and returns the first/last event arrival times + count.
func streamSSE(host, path string) (first, last time.Time, n int, err error) {
	req, _ := http.NewRequest("GET", edgeURL+path, nil)
	req.Host = host
	resp, err := httpc.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: event") {
			continue
		}
		now := time.Now()
		if n == 0 {
			first = now
		}
		last = now
		n++
	}
	return
}

// wsEcho opens a WebSocket through Caddy (Host header set via the URL, connection
// forced to the edge) and returns the echo of msg.
func wsEcho(host, path, msg string) (string, error) {
	dialer := &net.Dialer{}
	hc := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", strings.TrimPrefix(edgeURL, "http://"))
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws://"+host+path, &websocket.DialOptions{HTTPClient: hc})
	if err != nil {
		return "", fmt.Errorf("ws dial: %w", err)
	}
	defer c.CloseNow()
	if err := c.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		return "", err
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ── ward api + framework ─────────────────────────────────────────────────────────

func api(method, path string, body any) (int, []byte) {
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, wardAPI+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func setup() {
	api("POST", "/auth/setup", map[string]any{"username": "owner", "password": "supersecret1"})
	st, b := api("POST", "/auth/login", map[string]any{"username": "owner", "password": "supersecret1"})
	if st != 200 {
		fmt.Printf("login failed: %d %s\n", st, trim(string(b)))
		os.Exit(1)
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(b, &out)
	token = out.Token
}

func waitReady() {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := api("GET", "/auth/state", nil)
		if st == 200 {
			fmt.Println("ward ready")
			return
		}
		time.Sleep(time.Second)
	}
	fmt.Println("ward never became ready")
	os.Exit(1)
}

func check(name string, fn func() error) {
	if err := fn(); err != nil {
		fmt.Printf("FAIL  %-26s %v\n", name, err)
		failures++
		return
	}
	fmt.Printf("ok    %s\n", name)
}

func retry(attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return err
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func basic(u, p string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}
