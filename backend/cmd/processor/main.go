package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tf-vishal/zeotap-ims/internal/api/handlers"
	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/db"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	"github.com/tf-vishal/zeotap-ims/internal/processor"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
)

func main() {
	// ── 1. Configuration ──────────────────────────────────────────────
	cfg := config.Load()

	// ── 2. Redis ──────────────────────────────────────────────────────
	redisClient, err := iredis.NewClient(cfg)
	if err != nil {
		log.Fatalf("[processor] failed to connect to redis: %v", err)
	}
	defer redisClient.Close()

	// ── 3. MongoDB ────────────────────────────────────────────────────
	mongoClient, err := db.NewMongoClient(cfg)
	if err != nil {
		log.Fatalf("[processor] failed to connect to mongodb: %v", err)
	}
	defer mongoClient.Close(context.Background())

	// ── 4. PostgreSQL ─────────────────────────────────────────────────
	pgClient, err := db.NewPostgresClient(cfg)
	if err != nil {
		log.Fatalf("[processor] failed to connect to postgresql: %v", err)
	}
	defer pgClient.Close()

	// ── 5. Observability ──────────────────────────────────────────────
	// [CHANGE THIS] To change the Observability reporting interval (e.g. print metrics every X seconds),
	// modify the duration value here (currently 5 * time.Second).
	metrics := observability.NewProcessorMetrics(5*time.Second, redisClient)

	// ── 6. Debouncer ──────────────────────────────────────────────────
	debouncer := processor.NewDebouncer(
		redisClient, mongoClient, pgClient, metrics, cfg.DebounceWindowSec,
	)
	defer debouncer.Stop()

	// ── 7. Worker Pool ────────────────────────────────────────────────
	workerPool, err := processor.NewWorkerPool(
		redisClient, debouncer,
		cfg.RedisStreamName, cfg.ConsumerGroup, cfg.ConsumerNamePrefix,
		cfg.WorkerCount,
	)
	if err != nil {
		log.Fatalf("[processor] failed to create worker pool: %v", err)
	}

	workerPool.Start()
	defer workerPool.Stop()

	// ── 8. Health Endpoint (lightweight HTTP server) ──────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	healthHandler := &handlers.WorkerHealthHandler{
		RedisClient: redisClient,
		MongoClient: mongoClient,
		PGClient:    pgClient,
		WorkerPool:  workerPool,
	}
	router.GET("/health", healthHandler.HealthCheck)

	srv := &http.Server{
		Addr:    ":" + cfg.ProcessorPort,
		Handler: router,
	}

	go func() {
		log.Printf("[processor] health endpoint on :%s", cfg.ProcessorPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[processor] http server error: %v", err)
		}
	}()

	// ── 9. Graceful Shutdown ──────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[processor] received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("[processor] stopped cleanly")
}
