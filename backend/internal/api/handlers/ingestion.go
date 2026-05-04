package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tf-vishal/zeotap-ims/internal/models"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// IngestionHandler holds dependencies for the signal ingestion endpoint.
type IngestionHandler struct {
	Buffer  *iredis.StreamBuffer
	Metrics *observability.Metrics
}

// IngestSignal handles POST /api/v1/signals.
//
// Flow: validate JSON → push to async buffer → increment metric → return 202.
// No database calls, no blocking I/O in the hot path.
func (h *IngestionHandler) IngestSignal(c *gin.Context) {
	var signal models.Signal

	// Gin's ShouldBindJSON validates required fields via binding tags.
	if err := c.ShouldBindJSON(&signal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid payload",
			"details": err.Error(),
		})
		return
	}

	// Push to the async channel buffer.
	if !h.Buffer.Push(&signal) {
		// Channel full — system is overwhelmed; reject gracefully.
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "buffer full",
			"message": "ingestion buffer is saturated, please retry later",
		})
		return
	}

	// Increment the throughput counter (lock-free atomic operation).
	h.Metrics.IncrementIngested()

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "accepted",
		"message": "signal buffered for processing",
	})
}
