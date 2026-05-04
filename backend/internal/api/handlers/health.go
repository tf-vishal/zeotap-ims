package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// HealthHandler holds dependencies for the health check endpoint.
type HealthHandler struct {
	RedisClient *iredis.Client
}

// HealthCheck handles GET /health.
//
// Verifies:
//   - The service itself is running (implicit — if we respond, we're alive).
//   - Redis connection is healthy (active PING).
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	redisStatus := "healthy"
	if err := h.RedisClient.Ping(ctx); err != nil {
		redisStatus = "unhealthy: " + err.Error()
	}

	overallStatus := "healthy"
	httpStatus := http.StatusOK
	if redisStatus != "healthy" {
		overallStatus = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status": overallStatus,
		"checks": gin.H{
			"service": "running",
			"redis":   redisStatus,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
