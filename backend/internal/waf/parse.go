// Package waf parses Coraza JSON audit records into waf events and tails the
// audit log to ingest them. The parser is a Go port of spike/parse_audit.py.
package waf

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bhagyajitjagdev/ward/backend/internal/model"
)

var (
	// [key "value"] — value may contain escaped quotes
	tokenRe = regexp.MustCompile(`\[(\w+) "((?:[^"\\]|\\.)*)"\]`)
	// "... found within <TARGET>: <value>" where TARGET is like ARGS:id or REQUEST_URI
	dataRe = regexp.MustCompile(`found within ([A-Z_]+(?::[^:]+)?): (.*)$`)
)

type auditRecord struct {
	Transaction struct {
		ID            string `json:"id"`
		UnixTimestamp int64  `json:"unix_timestamp"`
		ClientIP      string `json:"client_ip"`
		Request       struct {
			Method  string              `json:"method"`
			URI     string              `json:"uri"`
			Headers map[string][]string `json:"headers"`
		} `json:"request"`
		Response struct {
			Status int `json:"status"`
		} `json:"response"`
		Producer struct {
			RuleEngine string `json:"rule_engine"`
		} `json:"producer"`
		IsInterrupted bool `json:"is_interrupted"`
	} `json:"transaction"`
	Messages []struct {
		ErrorMessage string `json:"error_message"`
	} `json:"messages"`
}

// ParseAuditRecord parses one Coraza JSON audit record into waf events — one per
// triggered rule (messages[]). Records with no messages yield nothing.
func ParseAuditRecord(raw []byte) ([]model.WAFEvent, error) {
	var rec auditRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	tx := rec.Transaction
	h := tx.Request.Headers
	base := model.WAFEvent{
		TxID:          tx.ID,
		TS:            time.Unix(0, tx.UnixTimestamp).UTC(),
		Host:          stripPort(firstHeader(h, "host")),
		ClientIP:      tx.ClientIP,
		Authed:        hasHeader(h, "cookie") || hasHeader(h, "authorization"),
		UserAgent:     firstHeader(h, "user-agent"),
		Method:        tx.Request.Method,
		Path:          pathOf(tx.Request.URI),
		URI:           tx.Request.URI,
		Status:        tx.Response.Status,
		EngineMode:    tx.Producer.RuleEngine,
		IsInterrupted: tx.IsInterrupted,
	}

	out := make([]model.WAFEvent, 0, len(rec.Messages))
	for _, m := range rec.Messages {
		ev := base
		fillFromErrorMessage(&ev, m.ErrorMessage)
		ev.Raw = m.ErrorMessage
		out = append(out, ev)
	}
	return out, nil
}

// fillFromErrorMessage extracts rule fields from the ModSecurity `[key "val"]` string.
func fillFromErrorMessage(ev *model.WAFEvent, msg string) {
	fields := map[string]string{}
	var tags []string
	for _, m := range tokenRe.FindAllStringSubmatch(msg, -1) {
		if m[1] == "tag" {
			tags = append(tags, m[2])
		} else {
			fields[m[1]] = m[2]
		}
	}
	if id, err := strconv.Atoi(fields["id"]); err == nil {
		ev.RuleID = id
	}
	ev.RuleMsg = fields["msg"]
	ev.Severity = fields["severity"]
	ev.CRSVersion = fields["ver"]
	ev.Tags = tags
	ev.IsAnomalyScore = contains(tags, "anomaly-evaluation")
	if dm := dataRe.FindStringSubmatch(fields["data"]); dm != nil {
		ev.MatchedTarget = dm[1]
		ev.MatchedValue = dm[2]
	}
}

func firstHeader(h map[string][]string, key string) string {
	for k, v := range h {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func hasHeader(h map[string][]string, key string) bool {
	for k := range h {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func stripPort(host string) string {
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.Contains(host, "]") {
		return host[:i]
	}
	return host
}

func pathOf(uri string) string {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		return uri[:i]
	}
	return uri
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
