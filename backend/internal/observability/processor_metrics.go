package observability

import (
	"log"
	"sync/atomic"
	"time"
)

// ProcessorMetrics provides thread-safe throughput tracking for the processing layer.
type ProcessorMetrics struct {
	processed        int64 // atomic — total signals processed since last reset
	workItemsCreated int64 // atomic — total work items created since last reset
}

// NewProcessorMetrics creates a ProcessorMetrics instance and starts the reporting loop.
func NewProcessorMetrics(interval time.Duration) *ProcessorMetrics {
	m := &ProcessorMetrics{}
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

// reportLoop runs every `interval`, logs throughput, and resets counters.
func (m *ProcessorMetrics) reportLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		processed := atomic.SwapInt64(&m.processed, 0)
		created := atomic.SwapInt64(&m.workItemsCreated, 0)
		seconds := interval.Seconds()

		log.Printf("[processor-metrics] throughput — processed_signals=%d (%.2f/sec) work_items_created=%d (%.2f/sec)",
			processed, float64(processed)/seconds,
			created, float64(created)/seconds)
	}
}
