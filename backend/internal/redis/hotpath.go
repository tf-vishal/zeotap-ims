package redis

import (
	"context"
	"fmt"
	"strconv"

	goredis "github.com/redis/go-redis/v9"
)

// Hot-path Redis keys for the live dashboard.
const (
	LiveIncidentsKey  = "ims:live_incidents"   // ZSET: incident_id scored by last_seen_at
	IncidentKeyPrefix = "ims:incident:"        // HASH: per-incident metadata
	MetricsKeyPrefix  = "ims:metrics:"         // STRING: global vitals
)

// LiveIncident represents a single incident in the hot-path feed.
type LiveIncident struct {
	ID          string `json:"id"`
	ComponentID string `json:"component_id"`
	Status      string `json:"status"`
	SignalCount int    `json:"signal_count"`
	FirstSeenAt string `json:"first_seen_at"`
	LastSeenAt  string `json:"last_seen_at"`
	Score       float64 `json:"score"` // Unix timestamp for sorting
}

// UpdateHotPath atomically adds/updates an incident in the Redis hot-path.
// Called by the debouncer after flushing a work item to PostgreSQL.
func (c *Client) UpdateHotPath(ctx context.Context, id, componentID, status string, signalCount int, firstSeenAt, lastSeenAt string, scoreUnix float64) error {
	pipe := c.RDB.Pipeline()

	// ZADD: upsert the incident scored by last_seen_at timestamp.
	pipe.ZAdd(ctx, LiveIncidentsKey, goredis.Z{
		Score:  scoreUnix,
		Member: id,
	})

	// HSET: store precomputed metadata for sub-10ms UI rendering.
	incidentKey := IncidentKeyPrefix + id
	pipe.HSet(ctx, incidentKey, map[string]interface{}{
		"id":            id,
		"component_id":  componentID,
		"status":        status,
		"signal_count":  signalCount,
		"first_seen_at": firstSeenAt,
		"last_seen_at":  lastSeenAt,
	})

	_, err := pipe.Exec(ctx)
	return err
}

// GetLiveIncidents retrieves the most recent incidents from the hot-path,
// sorted by last_seen_at descending. Returns up to `limit` entries.
func (c *Client) GetLiveIncidents(ctx context.Context, limit int64) ([]LiveIncident, error) {
	// ZREVRANGE to get the most recent incident IDs.
	ids, err := c.RDB.ZRevRangeWithScores(ctx, LiveIncidentsKey, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return []LiveIncident{}, nil
	}

	incidents := make([]LiveIncident, 0, len(ids))
	for _, z := range ids {
		id := z.Member.(string)
		incidentKey := IncidentKeyPrefix + id

		data, err := c.RDB.HGetAll(ctx, incidentKey).Result()
		if err != nil || len(data) == 0 {
			continue
		}

		sc, _ := strconv.Atoi(data["signal_count"])
		incidents = append(incidents, LiveIncident{
			ID:          data["id"],
			ComponentID: data["component_id"],
			Status:      data["status"],
			SignalCount: sc,
			FirstSeenAt: data["first_seen_at"],
			LastSeenAt:  data["last_seen_at"],
			Score:       z.Score,
		})
	}

	return incidents, nil
}

// PurgeFromHotPath removes an incident from the hot-path (called on closure).
func (c *Client) PurgeFromHotPath(ctx context.Context, id string) error {
	pipe := c.RDB.Pipeline()
	pipe.ZRem(ctx, LiveIncidentsKey, id)
	pipe.Del(ctx, IncidentKeyPrefix+id)
	_, err := pipe.Exec(ctx)
	return err
}

// SetMetric sets a global metric value in Redis for the UI to consume.
func (c *Client) SetMetric(ctx context.Context, name string, value float64) error {
	return c.RDB.Set(ctx, MetricsKeyPrefix+name, fmt.Sprintf("%.2f", value), 0).Err()
}

// GetMetric retrieves a global metric value from Redis.
func (c *Client) GetMetric(ctx context.Context, name string) (string, error) {
	return c.RDB.Get(ctx, MetricsKeyPrefix+name).Result()
}
