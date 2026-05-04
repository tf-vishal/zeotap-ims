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

// migrate creates the work_items table if it doesn't exist.
func (p *PostgresClient) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS work_items (
		id            TEXT PRIMARY KEY,
		component_id  TEXT NOT NULL,
		status        TEXT NOT NULL DEFAULT 'open',
		signal_count  INTEGER NOT NULL DEFAULT 1,
		first_seen_at TIMESTAMPTZ NOT NULL,
		last_seen_at  TIMESTAMPTZ NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_work_items_component ON work_items(component_id);
	CREATE INDEX IF NOT EXISTS idx_work_items_status ON work_items(status);
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

// Ping checks PostgreSQL connectivity.
func (p *PostgresClient) Ping(ctx context.Context) error {
	return p.DB.PingContext(ctx)
}

// Close shuts down the connection pool.
func (p *PostgresClient) Close() error {
	return p.DB.Close()
}
