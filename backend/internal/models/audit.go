package models

import "time"

// SignalAudit is the raw signal record stored in MongoDB (append-only data lake).
type SignalAudit struct {
	ComponentID string                 `bson:"component_id" json:"component_id"`
	SignalType  string                 `bson:"signal_type"  json:"signal_type"`
	Severity    string                 `bson:"severity"     json:"severity"`
	Timestamp   string                 `bson:"timestamp"    json:"timestamp"`
	Metadata    map[string]interface{} `bson:"metadata"     json:"metadata"`
	ReceivedAt  time.Time              `bson:"received_at"  json:"received_at"`
	WorkItemID  string                 `bson:"work_item_id" json:"work_item_id"`
}
