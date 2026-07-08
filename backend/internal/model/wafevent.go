package model

import "time"

// WAFEvent is one Coraza rule-match (a transaction that trips N rules → N events),
// parsed from the JSON audit log. It's both the ingest shape and the API DTO.
type WAFEvent struct {
	ID             string    `json:"id"`
	TxID           string    `json:"tx_id"`
	TS             time.Time `json:"ts"`
	ServiceID      *string   `json:"service_id,omitempty"`
	Host           string    `json:"host"`
	ClientIP       string    `json:"client_ip"`
	Authed         bool      `json:"authed"`
	UserAgent      string    `json:"user_agent,omitempty"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	URI            string    `json:"uri"`
	Status         int       `json:"status"`
	EngineMode     string    `json:"engine_mode"`
	IsInterrupted  bool      `json:"is_interrupted"`
	RuleID         int       `json:"rule_id"`
	RuleMsg        string    `json:"rule_msg"`
	Severity       string    `json:"severity"`
	MatchedTarget  string    `json:"matched_target,omitempty"`
	MatchedValue   string    `json:"matched_value,omitempty"`
	Tags           []string  `json:"tags"`
	IsAnomalyScore bool      `json:"is_anomaly_score"`
	CRSVersion     string    `json:"crs_version,omitempty"`
	Raw            string    `json:"raw,omitempty"`
}

// WAFTrigger is a clustered detection — a (service, path, rule, target) group
// with counts. It's the "top triggers" view that drives manual tuning. Anomaly
// aggregators (949xxx) are excluded; they aren't tuning targets.
type WAFTrigger struct {
	ServiceID     *string   `json:"service_id,omitempty"`
	Host          string    `json:"host,omitempty"`
	Path          string    `json:"path"`
	RuleID        int       `json:"rule_id"`
	RuleMsg       string    `json:"rule_msg"`
	Severity      string    `json:"severity"`
	MatchedTarget string    `json:"matched_target,omitempty"`
	Hits          int       `json:"hits"`
	DistinctIPs   int       `json:"distinct_ips"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}
