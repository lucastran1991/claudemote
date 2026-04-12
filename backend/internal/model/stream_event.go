package model

import "encoding/json"

// StreamEvent is used for a first-pass unmarshal to detect event type.
// Raw is intentionally omitted from JSON unmarshal — only Type/Subtype are decoded
// in this pass; a second unmarshal into the concrete type follows when needed.
type StreamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

// ResultEvent is the terminal stream-json event emitted by `claude --output-format stream-json`.
// It is the sole source of truth for all job summary fields.
type ResultEvent struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMs   int     `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	SessionID    string  `json:"session_id"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	StopReason   string  `json:"stop_reason"`
}
