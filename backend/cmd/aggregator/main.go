package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/db"
)

// The Aggregator is a lightweight cron-like service that periodically calculates
// MTTR and error-rate metrics from the work_items table and stores them in
// the hourly_component_metrics table.
//
// It is fully decoupled from ingestion and processing — it only reads/writes PostgreSQL.
func main() {
	cfg := config.Load()

	pgClient, err := db.NewPostgresClient(cfg)
	if err != nil {
		log.Fatalf("[aggregator] failed to connect to postgresql: %v", err)
	}
	defer pgClient.Close()

	interval := time.Duration(cfg.AggregatorIntervalSec) * time.Second
	log.Printf("[aggregator] starting — interval=%s", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately on startup.
	runAggregation(pgClient)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runAggregation(pgClient)
		case sig := <-stopCh:
			log.Printf("[aggregator] received signal %s, stopping", sig)
			return
		}
	}
}

// runAggregation calculates MTTR per component for the current hour and upserts into hourly_component_metrics.
func runAggregation(pg *db.PostgresClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Truncate to the current hour for grouping.
	currentHour := time.Now().UTC().Truncate(time.Hour)

	// Query closed work items that were resolved in the current hour.
	query := `
		SELECT component_id,
		       COUNT(*) as cnt,
		       AVG(EXTRACT(EPOCH FROM (resolved_at - first_seen_at))) as avg_mttr
		FROM work_items
		WHERE status = 'closed'
		  AND resolved_at IS NOT NULL
		  AND resolved_at >= $1
		  AND resolved_at < $2
		GROUP BY component_id
	`
	nextHour := currentHour.Add(time.Hour)
	rows, err := pg.DB.QueryContext(ctx, query, currentHour, nextHour)
	if err != nil {
		log.Printf("[aggregator] query error: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var componentID string
		var incidentCount int
		var avgMTTR float64

		if err := rows.Scan(&componentID, &incidentCount, &avgMTTR); err != nil {
			log.Printf("[aggregator] scan error: %v", err)
			continue
		}

		if err := pg.UpsertHourlyMetric(ctx, currentHour, componentID, incidentCount, avgMTTR); err != nil {
			log.Printf("[aggregator] upsert error for %s: %v", componentID, err)
			continue
		}
		count++
	}

	// Also aggregate all-time MTTR (not just current hour) for components.
	allTimeQuery := `
		SELECT component_id,
		       COUNT(*) as cnt,
		       AVG(EXTRACT(EPOCH FROM (resolved_at - first_seen_at))) as avg_mttr
		FROM work_items
		WHERE status = 'closed' AND resolved_at IS NOT NULL
		GROUP BY component_id
	`
	allRows, err := pg.DB.QueryContext(ctx, allTimeQuery)
	if err != nil {
		log.Printf("[aggregator] all-time query error: %v", err)
		return
	}
	defer allRows.Close()

	allTimeCount := 0
	for allRows.Next() {
		var componentID string
		var cnt int
		var avgMTTR float64
		if err := allRows.Scan(&componentID, &cnt, &avgMTTR); err != nil {
			continue
		}
		allTimeCount += cnt
	}

	log.Printf("[aggregator] cycle complete — hour=%s components_updated=%d total_closed=%d",
		currentHour.Format(time.RFC3339), count, allTimeCount)

	_ = fmt.Sprintf("") // prevent unused import
}
