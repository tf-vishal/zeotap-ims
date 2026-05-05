package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// HealthHandler holds dependencies for the health check endpoint.
type HealthHandler struct {
	RedisClient *iredis.Client
	MongoClient *db.MongoClient
	PGClient    *db.PostgresClient
}

// HealthCheck handles GET /health.
// Pings Redis, MongoDB, and PostgreSQL and returns individual + overall status.
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	redisOK := true
	mongoOK := true
	pgOK := true

	if err := h.RedisClient.Ping(ctx); err != nil {
		redisOK = false
	}
	if h.MongoClient != nil {
		if err := h.MongoClient.Ping(ctx); err != nil {
			mongoOK = false
		}
	}
	if h.PGClient != nil {
		if err := h.PGClient.Ping(ctx); err != nil {
			pgOK = false
		}
	}

	overall := "healthy"
	httpStatus := http.StatusOK
	if !redisOK || !mongoOK || !pgOK {
		overall = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	status := func(ok bool) string {
		if ok {
			return "up"
		}
		return "down"
	}

	c.JSON(httpStatus, gin.H{
		"status": overall,
		"services": gin.H{
			"redis":    status(redisOK),
			"mongodb":  status(mongoOK),
			"postgres": status(pgOK),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
