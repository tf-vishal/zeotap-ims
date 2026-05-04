package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/models"
)

// PostgresClient wraps the PostgreSQL connection pool and provides domain-specific methods.
type PostgresClient struct {
	DB *sql.DB
}

// NewPostgresClient opens a connection pool and runs schema migration.
func NewPostgresClient(cfg *config.Config) (*PostgresClient, error) {
	db, err := sql.Open("postgres", cfg.PostgresURI)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	// Pool tuning for concurrent workers
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	pc := &PostgresClient{DB: db}
	if err := pc.migrate(ctx); err != nil {
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}

	log.Println("[postgres] connected and migrated")
	return pc, nil
}

// migrate creates/updates tables. Additive migrations only.
func (p *PostgresClient) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS work_items (
		id            TEXT PRIMARY KEY,
		component_id  TEXT NOT NULL,
		status        TEXT NOT NULL DEFAULT 'open',
		signal_count  INTEGER NOT NULL DEFAULT 1,
		first_seen_at TIMESTAMPTZ NOT NULL,
		last_seen_at  TIMESTAMPTZ NOT NULL,
		resolved_at   TIMESTAMPTZ,
		rca_notes     TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_work_items_component ON work_items(component_id);
	CREATE INDEX IF NOT EXISTS idx_work_items_status ON work_items(status);

	-- Additive column migrations (safe to re-run)
	DO $$ BEGIN
		ALTER TABLE work_items ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;
		ALTER TABLE work_items ADD COLUMN IF NOT EXISTS rca_notes TEXT;
	EXCEPTION WHEN duplicate_column THEN NULL;
	END $$;

	-- Aggregation table for MTTR time-series reporting
	CREATE TABLE IF NOT EXISTS hourly_component_metrics (
		id              SERIAL PRIMARY KEY,
		hour            TIMESTAMPTZ NOT NULL,
		component_id    TEXT NOT NULL,
		incident_count  INTEGER NOT NULL DEFAULT 0,
		avg_mttr_seconds DOUBLE PRECISION,
		UNIQUE(hour, component_id)
	);
	CREATE INDEX IF NOT EXISTS idx_hcm_hour ON hourly_component_metrics(hour);
	CREATE INDEX IF NOT EXISTS idx_hcm_component ON hourly_component_metrics(component_id);
	`
	_, err := p.DB.ExecContext(ctx, schema)
	return err
}

// InsertWorkItem creates a new work item (called when a debounce window flushes).
func (p *PostgresClient) InsertWorkItem(ctx context.Context, item *models.WorkItem) error {
	query := `
		INSERT INTO work_items (id, component_id, status, signal_count, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := p.DB.ExecContext(ctx, query,
		item.ID, item.ComponentID, item.Status,
		item.SignalCount, item.FirstSeenAt, item.LastSeenAt,
	)
	return err
}

// GetWorkItem retrieves a single work item by ID.
func (p *PostgresClient) GetWorkItem(ctx context.Context, id string) (*models.WorkItem, error) {
	query := `SELECT id, component_id, status, signal_count, first_seen_at, last_seen_at, resolved_at, rca_notes
	          FROM work_items WHERE id = $1`
	row := p.DB.QueryRowContext(ctx, query, id)

	var item models.WorkItem
	var resolvedAt sql.NullTime
	var rcaNotes sql.NullString

	err := row.Scan(&item.ID, &item.ComponentID, &item.Status, &item.SignalCount,
		&item.FirstSeenAt, &item.LastSeenAt, &resolvedAt, &rcaNotes)
	if err != nil {
		return nil, err
	}
	if resolvedAt.Valid {
		item.ResolvedAt = &resolvedAt.Time
	}
	if rcaNotes.Valid {
		item.RCANotes = rcaNotes.String
	}
	return &item, nil
}

// UpdateWorkItemStatus transitions the status of a work item (but NOT to 'closed' — use CloseWorkItem for that).
func (p *PostgresClient) UpdateWorkItemStatus(ctx context.Context, id, status string) error {
	query := `UPDATE work_items SET status = $1 WHERE id = $2`
	_, err := p.DB.ExecContext(ctx, query, status, id)
	return err
}

// CloseWorkItem marks the work item as closed with mandatory RCA notes and a resolution timestamp.
func (p *PostgresClient) CloseWorkItem(ctx context.Context, id, rcaNotes string) error {
	query := `UPDATE work_items SET status = 'closed', resolved_at = $1, rca_notes = $2 WHERE id = $3`
	_, err := p.DB.ExecContext(ctx, query, time.Now().UTC(), rcaNotes, id)
	return err
}

// GetMTTRByComponent returns the average MTTR (in seconds) for each component from closed work items.
func (p *PostgresClient) GetMTTRByComponent(ctx context.Context) ([]models.ComponentMTTR, error) {
	query := `
		SELECT component_id,
		       COUNT(*) as total_incidents,
		       AVG(EXTRACT(EPOCH FROM (resolved_at - first_seen_at))) as avg_mttr_seconds
		FROM work_items
		WHERE status = 'closed' AND resolved_at IS NOT NULL
		GROUP BY component_id
		ORDER BY avg_mttr_seconds DESC
	`
	rows, err := p.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ComponentMTTR
	for rows.Next() {
		var m models.ComponentMTTR
		if err := rows.Scan(&m.ComponentID, &m.TotalIncidents, &m.AvgMTTRSeconds); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
}

// UpsertHourlyMetric inserts or updates an hourly metric row for the aggregator.
func (p *PostgresClient) UpsertHourlyMetric(ctx context.Context, hour time.Time, componentID string, incidentCount int, avgMTTR float64) error {
	query := `
		INSERT INTO hourly_component_metrics (hour, component_id, incident_count, avg_mttr_seconds)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (hour, component_id)
		DO UPDATE SET incident_count = $3, avg_mttr_seconds = $4
	`
	_, err := p.DB.ExecContext(ctx, query, hour, componentID, incidentCount, avgMTTR)
	return err
}

// GetHourlyMetrics retrieves recent hourly metrics for the analytics dashboard.
func (p *PostgresClient) GetHourlyMetrics(ctx context.Context, hours int) ([]models.HourlyMetric, error) {
	query := `
		SELECT hour, component_id, incident_count, avg_mttr_seconds
		FROM hourly_component_metrics
		WHERE hour >= NOW() - ($1 || ' hours')::INTERVAL
		ORDER BY hour DESC, component_id
	`
	rows, err := p.DB.QueryContext(ctx, query, fmt.Sprintf("%d", hours))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.HourlyMetric
	for rows.Next() {
		var m models.HourlyMetric
		var avgMTTR sql.NullFloat64
		if err := rows.Scan(&m.Hour, &m.ComponentID, &m.IncidentCount, &avgMTTR); err != nil {
			return nil, err
		}
		if avgMTTR.Valid {
			m.AvgMTTRSeconds = avgMTTR.Float64
		}
		results = append(results, m)
	}
	return results, nil
}

// Ping checks PostgreSQL connectivity.
func (p *PostgresClient) Ping(ctx context.Context) error {
	return p.DB.PingContext(ctx)
}

// Close shuts down the connection pool.
func (p *PostgresClient) Close() error {
	return p.DB.Close()
}
