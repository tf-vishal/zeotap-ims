package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/tf-vishal/zeotap-ims/internal/api/handlers"
	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
	"golang.org/x/time/rate"
)

func main() {
	// ── 1. Configuration ──────────────────────────────────────────────
	cfg := config.Load()

	// ── 2. Redis ──────────────────────────────────────────────────────
	redisClient, err := iredis.NewClient(cfg)
	if err != nil {
		log.Fatalf("[main] failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	// ── 3. MongoDB (for signal investigation queries) ─────────────────
	mongoClient, err := db.NewMongoClient(cfg)
	if err != nil {
		log.Fatalf("[main] failed to connect to mongodb: %v", err)
	}
	defer mongoClient.Close(context.Background())

	// ── 4. PostgreSQL (for incident queries and closure) ──────────────
	pgClient, err := db.NewPostgresClient(cfg)
	if err != nil {
		log.Fatalf("[main] failed to connect to postgresql: %v", err)
	}
	defer pgClient.Close()

	// ── 5. Stream Buffer (async) ──────────────────────────────────────
	// 4 background workers drain the channel into Redis streams.
	streamBuffer := iredis.NewStreamBuffer(redisClient, cfg.RedisStreamName, 4)

	// ── 6. Observability ──────────────────────────────────────────────
	metrics := observability.NewMetrics(5*time.Second, redisClient)

	// ── 7. Gin Router ─────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// CORS — allow the React frontend to call the API.
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Rate limiter will be injected into the ingestion handler to limit per signal.
	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.RateBurst)

	// ── 8. Handlers ───────────────────────────────────────────────────
	ingestionHandler := &handlers.IngestionHandler{
		Buffer:  streamBuffer,
		Metrics: metrics,
		Limiter: limiter,
	}
	healthHandler := &handlers.HealthHandler{
		RedisClient: redisClient,
		MongoClient: mongoClient,
		PGClient:    pgClient,
	}
	incidentHandler := &handlers.IncidentHandler{
		RedisClient: redisClient,
		MongoClient: mongoClient,
		PGClient:    pgClient,
	}

	// ── Stage 1: Ingestion ────────────────────────────────────────────
	router.POST("/api/v1/signals", ingestionHandler.IngestSignal)
	router.GET("/health", healthHandler.HealthCheck)

	// ── Stage 5: Workflow API ─────────────────────────────────────────
	router.GET("/api/v1/incidents/live", incidentHandler.GetLiveIncidents)
	router.GET("/api/v1/incidents/:id/signals", incidentHandler.GetIncidentSignals)
	router.PATCH("/api/v1/incidents/:id", incidentHandler.UpdateIncidentStatus)
	router.POST("/api/v1/incidents/:id/close", incidentHandler.CloseIncident)
	router.GET("/api/v1/analytics/vitals", incidentHandler.GetVitals)

	// ── 9. HTTP Server ────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// ── 10. Graceful Shutdown ─────────────────────────────────────────
	go func() {
		log.Printf("[main] server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[main] received signal %s, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[main] forced shutdown: %v", err)
	}

	log.Println("[main] server stopped cleanly")
}
