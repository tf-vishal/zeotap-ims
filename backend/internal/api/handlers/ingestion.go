package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/time/rate"
	"github.com/tf-vishal/zeotap-ims/internal/models"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// IngestionHandler holds dependencies for the signal ingestion endpoint.
type IngestionHandler struct {
	Buffer  *iredis.StreamBuffer
	Metrics *observability.Metrics
	Limiter *rate.Limiter
}

// IngestSignal handles POST /api/v1/signals.
//
// Flow: validate JSON → push to async buffer → increment metric → return 202.
// No database calls, no blocking I/O in the hot path.
func (h *IngestionHandler) IngestSignal(c *gin.Context) {
	// Check if the payload is a single object or an array
	// Using ShouldBindBodyWith to read body multiple times if needed, or just bind to a slice
	var signals []models.Signal

	// Try binding as an array first
	if err := c.ShouldBindBodyWith(&signals, binding.JSON); err != nil {
		// If array binding fails, try binding as a single object
		var singleSignal models.Signal
		if err := c.ShouldBindBodyWith(&singleSignal, binding.JSON); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid payload format",
				"details": err.Error(),
			})
			return
		}
		signals = append(signals, singleSignal)
	}

	// Enforce rate limit based on the NUMBER of signals
	if !h.Limiter.AllowN(time.Now(), len(signals)) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":   "rate limit exceeded",
			"message": "server is under heavy load, please retry later",
		})
		return
	}

	acceptedCount := 0
	for i := range signals {
		// Push to the async channel buffer.
		if h.Buffer.Push(&signals[i]) {
			acceptedCount++
			// Increment the throughput counter (lock-free atomic operation).
			h.Metrics.IncrementIngested()
		}
	}

	if acceptedCount == 0 {
		// Channel full — system is overwhelmed; reject gracefully.
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "buffer full",
			"message": "ingestion buffer is saturated, please retry later",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":   "accepted",
		"message":  "signals buffered for processing",
		"accepted": acceptedCount,
	})
}
