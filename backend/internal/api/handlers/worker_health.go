package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	"github.com/tf-vishal/zeotap-ims/internal/processor"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// WorkerHealthHandler holds dependencies for the processor health endpoint.
type WorkerHealthHandler struct {
	RedisClient *iredis.Client
	MongoClient *db.MongoClient
	PGClient    *db.PostgresClient
	WorkerPool  *processor.WorkerPool
}

// HealthCheck handles GET /health for the processor service.
//
// Verifies:
//   - Service is running (implicit).
//   - Redis connectivity.
//   - MongoDB connectivity.
//   - PostgreSQL connectivity.
//   - Consumer lag (pending messages in stream).
func (h *WorkerHealthHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	// ── Check Redis ──────────────────────────────────────────────────
	redisStatus := "healthy"
	if err := h.RedisClient.Ping(ctx); err != nil {
		redisStatus = "unhealthy: " + err.Error()
	}

	// ── Check MongoDB ────────────────────────────────────────────────
	mongoStatus := "healthy"
	if err := h.MongoClient.Ping(ctx); err != nil {
		mongoStatus = "unhealthy: " + err.Error()
	}

	// ── Check PostgreSQL ─────────────────────────────────────────────
	pgStatus := "healthy"
	if err := h.PGClient.Ping(ctx); err != nil {
		pgStatus = "unhealthy: " + err.Error()
	}

	// ── Consumer Lag ─────────────────────────────────────────────────
	var consumerLag int64 = -1
	if pending, err := h.WorkerPool.GetPendingCount(ctx); err == nil {
		consumerLag = pending
	}

	// ── Overall Status ───────────────────────────────────────────────
	overallStatus := "healthy"
	httpStatus := http.StatusOK
	if redisStatus != "healthy" || mongoStatus != "healthy" || pgStatus != "healthy" {
		overallStatus = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status": overallStatus,
		"checks": gin.H{
			"service":    "running",
			"redis":      redisStatus,
			"mongodb":    mongoStatus,
			"postgresql": pgStatus,
		},
		"consumer_lag": consumerLag,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	})
}
