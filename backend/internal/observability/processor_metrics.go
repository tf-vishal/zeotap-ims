package observability

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// ProcessorMetrics provides thread-safe throughput tracking for the processing layer.
// It publishes processing_per_sec to Redis for the dashboard.
type ProcessorMetrics struct {
	processed        int64 // atomic — total signals processed since last reset
	workItemsCreated int64 // atomic — total work items created since last reset
	redisClient      *iredis.Client
}

// NewProcessorMetrics creates a ProcessorMetrics instance and starts the reporting loop.
func NewProcessorMetrics(interval time.Duration, redisClient *iredis.Client) *ProcessorMetrics {
	m := &ProcessorMetrics{redisClient: redisClient}
	go m.reportLoop(interval)
	log.Printf("[processor-metrics] throughput reporter started — interval=%s", interval)
	return m
}

// IncrementProcessed atomically increments the processed counter.
func (m *ProcessorMetrics) IncrementProcessed() {
	atomic.AddInt64(&m.processed, 1)
}

// IncrementWorkItemsCreated atomically increments the work items created counter.
func (m *ProcessorMetrics) IncrementWorkItemsCreated() {
	atomic.AddInt64(&m.workItemsCreated, 1)
}

// reportLoop runs every `interval`, logs throughput, resets counters, and pushes to Redis.
func (m *ProcessorMetrics) reportLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		processed := atomic.SwapInt64(&m.processed, 0)
		created := atomic.SwapInt64(&m.workItemsCreated, 0)
		seconds := interval.Seconds()

		processedRate := float64(processed) / seconds
		createdRate := float64(created) / seconds

		log.Printf("[processor-metrics] throughput — processed_signals=%d (%.2f/sec) work_items_created=%d (%.2f/sec)",
			processed, processedRate, created, createdRate)

		// Publish to Redis for the dashboard.
		if m.redisClient != nil {
			m.redisClient.SetMetric(context.Background(), "processing_per_sec", processedRate)
			m.redisClient.SetMetric(context.Background(), "work_items_per_sec", createdRate)
		}
	}
}
