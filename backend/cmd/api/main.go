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
	"github.com/tf-vishal/zeotap-ims/internal/api/middleware"
	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/observability"
	iredis "github.com/tf-vishal/zeotap-ims/internal/redis"
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

	// ── 3. Stream Buffer (async) ──────────────────────────────────────
	// 4 background workers drain the channel into Redis streams.
	streamBuffer := iredis.NewStreamBuffer(redisClient, cfg.RedisStreamName, 4)

	// ── 4. Observability ──────────────────────────────────────────────
	metrics := observability.NewMetrics(5 * time.Second)

	// ── 5. Gin Router ─────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery()) // panic recovery middleware

	// Global rate limiter — applied to all routes.
	router.Use(middleware.RateLimiter(cfg.RateLimit, cfg.RateBurst))

	// ── 6. Handlers ───────────────────────────────────────────────────
	ingestionHandler := &handlers.IngestionHandler{
		Buffer:  streamBuffer,
		Metrics: metrics,
	}
	healthHandler := &handlers.HealthHandler{
		RedisClient: redisClient,
	}

	router.POST("/api/v1/signals", ingestionHandler.IngestSignal)
	router.GET("/health", healthHandler.HealthCheck)

	// ── 7. HTTP Server ────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// ── 8. Graceful Shutdown ──────────────────────────────────────────
	// Start server in a goroutine so we can listen for OS signals.
	go func() {
		log.Printf("[main] server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	// Block until SIGINT or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[main] received signal %s, shutting down gracefully...", sig)

	// Give in-flight requests 10 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("[main] forced shutdown: %v", err)
	}

	log.Println("[main] server stopped cleanly")
}
