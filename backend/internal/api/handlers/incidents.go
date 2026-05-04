package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

// IncidentHandler holds dependencies for incident workflow endpoints.
type IncidentHandler struct {
	RedisClient *iredis.Client
	MongoClient *db.MongoClient
	PGClient    *db.PostgresClient
}

// GetLiveIncidents handles GET /api/v1/incidents/live.
// Reads from the Redis Hot-Path (ZREVRANGE + HGETALL) for sub-10ms response.
func (h *IncidentHandler) GetLiveIncidents(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	incidents, err := h.RedisClient.GetLiveIncidents(ctx, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch live incidents", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"incidents": incidents,
		"count":     len(incidents),
	})
}

// GetIncidentSignals handles GET /api/v1/incidents/:id/signals.
// Queries MongoDB for the raw signal payloads for forensic analysis.
func (h *IncidentHandler) GetIncidentSignals(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	signals, err := h.MongoClient.GetSignalsByWorkItemID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch signals", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"incident_id": id,
		"signals":     signals,
		"count":       len(signals),
	})
}

// CloseIncidentRequest is the payload for the RCA closure form.
type CloseIncidentRequest struct {
	RCANotes string `json:"rca_notes" binding:"required"`
}

// CloseIncident handles POST /api/v1/incidents/:id/close.
// Enforces mandatory RCA notes, updates PostgreSQL, and purges from the Redis Hot-Path.
func (h *IncidentHandler) CloseIncident(c *gin.Context) {
	id := c.Param("id")

	var req CloseIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "RCA notes are mandatory for incident closure",
			"details": err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Verify the incident exists in Postgres.
	item, err := h.PGClient.GetWorkItem(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "incident not found", "details": err.Error()})
		return
	}

	if item.Status == "closed" {
		c.JSON(http.StatusConflict, gin.H{"error": "incident is already closed"})
		return
	}

	// Close in PostgreSQL with RCA notes and resolved_at.
	if err := h.PGClient.CloseWorkItem(ctx, id, req.RCANotes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close incident", "details": err.Error()})
		return
	}

	// Purge from Redis Hot-Path — it's no longer "live".
	if err := h.RedisClient.PurgeFromHotPath(ctx, id); err != nil {
		// Log but don't fail — PG is the source of truth.
		c.JSON(http.StatusOK, gin.H{
			"status":  "closed",
			"warning": "hot-path purge failed, will be cleaned up by scanner",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "closed",
		"incident_id": id,
		"message":     "incident closed with RCA notes",
	})
}

// UpdateStatusRequest is the payload for status transitions (not closure).
type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateIncidentStatus handles PATCH /api/v1/incidents/:id.
// Allows transitions: open → investigating, investigating → resolved.
// Does NOT allow transition to "closed" — use POST /close with RCA for that.
func (h *IncidentHandler) UpdateIncidentStatus(c *gin.Context) {
	id := c.Param("id")

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	// Prevent using this endpoint to close — must use /close with RCA.
	if req.Status == "closed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cannot close via status update — use POST /api/v1/incidents/:id/close with RCA notes",
		})
		return
	}

	allowed := map[string]bool{"investigating": true, "resolved": true, "open": true}
	if !allowed[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status. allowed: open, investigating, resolved"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := h.PGClient.UpdateWorkItemStatus(ctx, id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status", "details": err.Error()})
		return
	}

	// Update hot-path status as well.
	h.RedisClient.RDB.HSet(ctx, iredis.IncidentKeyPrefix+id, "status", req.Status)

	c.JSON(http.StatusOK, gin.H{
		"status":      req.Status,
		"incident_id": id,
	})
}

// GetVitals handles GET /api/v1/analytics/vitals.
// Returns global system metrics and MTTR per component.
func (h *IncidentHandler) GetVitals(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Read global metrics from Redis.
	signalsPerSec, _ := h.RedisClient.GetMetric(ctx, "signals_per_sec")
	processingPerSec, _ := h.RedisClient.GetMetric(ctx, "processing_per_sec")

	// Get consumer lag (pending messages).
	var consumerLag int64 = -1
	pending, err := h.RedisClient.RDB.XPending(ctx, "ims:signals", "ims-processor-group").Result()
	if err == nil {
		consumerLag = pending.Count
	}

	// Get MTTR by component from PostgreSQL.
	mttr, err := h.PGClient.GetMTTRByComponent(ctx)
	if err != nil {
		mttr = nil
	}

	// Get hourly metrics for the last 24 hours.
	hourly, err := h.PGClient.GetHourlyMetrics(ctx, 24)
	if err != nil {
		hourly = nil
	}

	// Cache hit check: try getting a known key to measure.
	var cacheStatus string
	if _, err := h.RedisClient.RDB.Get(ctx, iredis.MetricsKeyPrefix+"signals_per_sec").Result(); err == goredis.Nil {
		cacheStatus = "cold"
	} else if err != nil {
		cacheStatus = "error"
	} else {
		cacheStatus = "hot"
	}

	c.JSON(http.StatusOK, gin.H{
		"signals_per_sec":    signalsPerSec,
		"processing_per_sec": processingPerSec,
		"consumer_lag":       consumerLag,
		"cache_status":       cacheStatus,
		"mttr_by_component":  mttr,
		"hourly_metrics":     hourly,
	})
}
