package models

import "time"

// WorkItem represents an aggregated incident created from debounced signals.
// Stored in PostgreSQL as the transactional record.
type WorkItem struct {
	ID          string    `json:"id"`
	ComponentID string    `json:"component_id"`
	Status      string    `json:"status"` // open, investigating, resolved, closed
	SignalCount int       `json:"signal_count"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
