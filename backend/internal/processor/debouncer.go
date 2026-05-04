package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	"github.com/tf-vishal/zeotap-ims/internal/models"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

const (
	lockPrefix = "debounce:lock:"
	dataPrefix = "debounce:data:"
)

// Debouncer implements a sliding-window debounce with delayed flush to PostgreSQL.
//
// Architecture:
//   - Each component_id gets TWO Redis keys:
//     1. debounce:lock:{component_id}   — a marker key with TTL = debounce window.
//        Every new signal resets the TTL (sliding window). When this key expires,
//        the window is "closed".
//     2. debounce:data:{component_id}   — a hash storing the accumulated work item
//        data (work_item_id, signal_count, first_seen_at, last_seen_at). No TTL.
//
//   - A Redis keyspace notification subscriber listens for key expirations.
//     When a lock key expires, it reads the data key, flushes to PostgreSQL,
//     and deletes the data key.
//
//   - A background scanner runs periodically to catch any orphaned data keys
//     (in case a keyspace notification is missed — they are "fire and forget").
type Debouncer struct {
	redis       *iredis.Client
	mongo       *db.MongoClient
	postgres    *db.PostgresClient
	metrics     *observability.ProcessorMetrics
	windowSec   int
	stopCh      chan struct{}
}

// NewDebouncer creates a debouncer and starts the keyspace expiry listener + fallback scanner.
func NewDebouncer(
	redisClient *iredis.Client,
	mongoClient *db.MongoClient,
	pgClient *db.PostgresClient,
	metrics *observability.ProcessorMetrics,
	windowSec int,
) *Debouncer {
	d := &Debouncer{
		redis:    redisClient,
		mongo:    mongoClient,
		postgres: pgClient,
		metrics:  metrics,
		windowSec: windowSec,
		stopCh:   make(chan struct{}),
	}

	go d.listenForExpiredKeys()
	go d.fallbackScanner()

	return d
}

// ProcessSignal handles a single signal: debounce logic + MongoDB audit insert.
//
// Flow:
//  1. Check if a debounce lock exists for this component_id.
//  2. If NO lock: new debounce window — create UUID, init data hash, set lock with TTL.
//  3. If lock EXISTS: sliding window — increment counter, update last_seen_at, reset TTL.
//  4. Always: insert raw signal into MongoDB for audit trail.
func (d *Debouncer) ProcessSignal(ctx context.Context, signal *models.Signal) error {
	lockKey := lockPrefix + signal.ComponentID
	dataKey := dataPrefix + signal.ComponentID
	now := time.Now().UTC()

	// Attempt to SET the lock key only if it does NOT exist (NX).
	// If we successfully set it, this is a NEW debounce window.
	set, err := d.redis.RDB.SetNX(ctx, lockKey, "1", time.Duration(d.windowSec)*time.Second).Result()
	if err != nil {
		return fmt.Errorf("debounce SetNX: %w", err)
	}

	var workItemID string

	if set {
		// ── New debounce window ──────────────────────────────────────
		workItemID = uuid.New().String()

		// Initialize the data hash with all accumulated fields.
		pipe := d.redis.RDB.Pipeline()
		pipe.HSet(ctx, dataKey, map[string]interface{}{
			"work_item_id":  workItemID,
			"component_id":  signal.ComponentID,
			"signal_count":  1,
			"first_seen_at": now.Format(time.RFC3339Nano),
			"last_seen_at":  now.Format(time.RFC3339Nano),
		})
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("debounce pipeline (new): %w", err)
		}

		d.metrics.IncrementWorkItemsCreated()
		log.Printf("[debouncer] new window — component=%s work_item=%s", signal.ComponentID, workItemID)
	} else {
		// ── Existing debounce window — slide it ─────────────────────
		// Reset the TTL (sliding window behavior).
		d.redis.RDB.Expire(ctx, lockKey, time.Duration(d.windowSec)*time.Second)

		// Atomically increment signal_count and update last_seen_at.
		pipe := d.redis.RDB.Pipeline()
		pipe.HIncrBy(ctx, dataKey, "signal_count", 1)
		pipe.HSet(ctx, dataKey, "last_seen_at", now.Format(time.RFC3339Nano))
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("debounce pipeline (update): %w", err)
		}

		// Read the work_item_id for the audit record.
		workItemID, err = d.redis.RDB.HGet(ctx, dataKey, "work_item_id").Result()
		if err != nil {
			return fmt.Errorf("debounce HGet work_item_id: %w", err)
		}
	}

	// ── Always: insert raw signal into MongoDB ──────────────────────
	audit := &models.SignalAudit{
		ComponentID: signal.ComponentID,
		SignalType:  signal.SignalType,
		Severity:    signal.Severity,
		Timestamp:   signal.Timestamp,
		Metadata:    signal.Metadata,
		ReceivedAt:  now,
		WorkItemID:  workItemID,
	}
	if err := d.mongo.InsertSignalAudit(ctx, audit); err != nil {
		return fmt.Errorf("mongo insert audit: %w", err)
	}

	d.metrics.IncrementProcessed()
	return nil
}

// listenForExpiredKeys subscribes to Redis keyspace notifications for expired keys.
// When a debounce:lock:* key expires, it reads the corresponding data key,
// flushes to PostgreSQL, and cleans up.
func (d *Debouncer) listenForExpiredKeys() {
	// Subscribe to expired events on DB 0.
	// Requires Redis to be started with: notify-keyspace-events Ex
	pubsub := d.redis.RDB.PSubscribe(context.Background(), "__keyevent@0__:expired")
	defer pubsub.Close()

	log.Println("[debouncer] listening for keyspace expiry notifications")

	ch := pubsub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// msg.Payload is the key that expired, e.g. "debounce:lock:svc-auth"
			if strings.HasPrefix(msg.Payload, lockPrefix) {
				componentID := strings.TrimPrefix(msg.Payload, lockPrefix)
				d.flushWorkItem(componentID)
			}
		case <-d.stopCh:
			return
		}
	}
}

// fallbackScanner periodically checks for orphaned data keys without a corresponding lock.
// This catches any expired windows where the keyspace notification was missed.
func (d *Debouncer) fallbackScanner() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.scanOrphanedWindows()
		case <-d.stopCh:
			return
		}
	}
}

// scanOrphanedWindows finds data keys without a corresponding lock key and flushes them.
func (d *Debouncer) scanOrphanedWindows() {
	ctx := context.Background()
	var cursor uint64

	for {
		// Scan for all debounce:data:* keys.
		keys, nextCursor, err := d.redis.RDB.Scan(ctx, cursor, "debounce:data:*", 100).Result()
		if err != nil {
			log.Printf("[debouncer] scan error: %v", err)
			return
		}

		for _, dataKey := range keys {
			// Extract component_id from "debounce:data:{component_id}"
			if !strings.HasPrefix(dataKey, dataPrefix) {
				continue
			}
			componentID := strings.TrimPrefix(dataKey, dataPrefix)
			lockKey := lockPrefix + componentID

			// If the lock key is gone, the window has expired — flush it.
			exists, err := d.redis.RDB.Exists(ctx, lockKey).Result()
			if err != nil {
				log.Printf("[debouncer] exists check error for %s: %v", lockKey, err)
				continue
			}
			if exists == 0 {
				log.Printf("[debouncer] fallback flush for orphaned window: %s", componentID)
				d.flushWorkItem(componentID)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

// flushWorkItem reads the accumulated data from Redis, inserts into PostgreSQL, and cleans up.
func (d *Debouncer) flushWorkItem(componentID string) {
	ctx := context.Background()
	dataKey := dataPrefix + componentID

	// Read all fields from the data hash.
	data, err := d.redis.RDB.HGetAll(ctx, dataKey).Result()
	if err != nil || len(data) == 0 {
		// Key already cleaned up (another worker or scanner got it first).
		return
	}

	// Parse accumulated data.
	signalCount, _ := strconv.Atoi(data["signal_count"])
	firstSeenAt, _ := time.Parse(time.RFC3339Nano, data["first_seen_at"])
	lastSeenAt, _ := time.Parse(time.RFC3339Nano, data["last_seen_at"])

	workItem := &models.WorkItem{
		ID:          data["work_item_id"],
		ComponentID: data["component_id"],
		Status:      "open",
		SignalCount: signalCount,
		FirstSeenAt: firstSeenAt,
		LastSeenAt:  lastSeenAt,
	}

	if err := d.postgres.InsertWorkItem(ctx, workItem); err != nil {
		log.Printf("[debouncer] flush to postgres failed for %s: %v", componentID, err)
		// Don't delete the data key so the fallback scanner can retry.
		return
	}

	// Clean up the data key.
	d.redis.RDB.Del(ctx, dataKey)

	log.Printf("[debouncer] flushed work_item=%s component=%s signals=%d",
		workItem.ID, componentID, signalCount)
}

// Stop signals all background goroutines to shut down.
func (d *Debouncer) Stop() {
	close(d.stopCh)
}

// parseSignalFromPayload deserializes a signal from the Redis stream payload.
func ParseSignalFromPayload(payload string) (*models.Signal, error) {
	var signal models.Signal
	if err := json.Unmarshal([]byte(payload), &signal); err != nil {
		return nil, err
	}
	return &signal, nil
}
