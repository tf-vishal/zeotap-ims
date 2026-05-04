package models

// Signal represents a single incoming signal from a distributed system.
// All fields use JSON tags matching the API contract.
type Signal struct {
	ComponentID string                 `json:"component_id" binding:"required"`
	SignalType  string                 `json:"signal_type"  binding:"required"`
	Severity    string                 `json:"severity"     binding:"required"`
	Timestamp   string                 `json:"timestamp"    binding:"required"`
	Metadata    map[string]interface{} `json:"metadata"`
}
