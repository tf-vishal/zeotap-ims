package observability

import (
	"log"
	"sync/atomic"
	"time"
)

// Metrics provides thread-safe throughput tracking using atomic operations.
// A background goroutine logs signals/sec every reporting interval and resets the counter.
type Metrics struct {
	ingested int64 // atomic counter — total signals since last reset
}

// NewMetrics creates a Metrics instance and starts the reporting loop.
func NewMetrics(interval time.Duration) *Metrics {
	m := &Metrics{}
	go m.reportLoop(interval)
	log.Printf("[metrics] throughput reporter started — interval=%s", interval)
	return m
}

// IncrementIngested atomically increments the ingested counter.
// Called from the hot path — must be lock-free.
func (m *Metrics) IncrementIngested() {
	atomic.AddInt64(&m.ingested, 1)
}

// reportLoop runs every `interval`, logs throughput, and resets the counter.
func (m *Metrics) reportLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Swap the counter to zero and read the previous value atomically.
		count := atomic.SwapInt64(&m.ingested, 0)
		rate := float64(count) / interval.Seconds()
		log.Printf("[metrics] throughput — signals_ingested=%d window=%s signals_per_sec=%.2f",
			count, interval, rate)
	}
}
