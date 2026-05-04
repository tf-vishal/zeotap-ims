package models

import "time"

// WorkItem represents an aggregated incident created from debounced signals.
// Stored in PostgreSQL as the transactional record.
type WorkItem struct {
	ID          string     `json:"id"`
	ComponentID string     `json:"component_id"`
	Status      string     `json:"status"` // open, investigating, resolved, closed
	SignalCount int        `json:"signal_count"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	RCANotes    string     `json:"rca_notes,omitempty"`
}

// ComponentMTTR holds the MTTR analytics result for a single component.
type ComponentMTTR struct {
	ComponentID    string  `json:"component_id"`
	TotalIncidents int     `json:"total_incidents"`
	AvgMTTRSeconds float64 `json:"avg_mttr_seconds"`
}

// HourlyMetric holds a single row from the hourly_component_metrics table.
type HourlyMetric struct {
	Hour            time.Time `json:"hour"`
	ComponentID     string    `json:"component_id"`
	IncidentCount   int       `json:"incident_count"`
	AvgMTTRSeconds  float64   `json:"avg_mttr_seconds"`
}
