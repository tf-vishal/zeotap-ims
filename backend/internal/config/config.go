package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all environment-driven configuration for the ingestion service.
type Config struct {
	ServerPort      string
	RateLimit       int // tokens per second (refill rate)
	RateBurst       int // max burst size (bucket capacity)
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RedisStreamName string
}

// Load reads the .env file and populates a Config struct.
// It panics on missing critical values so the service fails fast at startup.
func Load() *Config {
	// Load .env file if present; ignore errors (env vars may be set externally).
	if err := godotenv.Load(); err != nil {
		log.Println("[config] no .env file found, relying on environment variables")
	}

	cfg := &Config{
		ServerPort:      getEnvOrDefault("SERVER_PORT", "8080"),
		RateLimit:       getEnvAsInt("RATE_LIMIT", 10000),
		RateBurst:       getEnvAsInt("RATE_BURST", 12000),
		RedisAddr:       getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnvOrDefault("REDIS_PASSWORD", ""),
		RedisDB:         getEnvAsInt("REDIS_DB", 0),
		RedisStreamName: getEnvOrDefault("REDIS_STREAM_NAME", "ims:signals"),
	}

	log.Printf("[config] loaded — port=%s rate=%d burst=%d redis=%s stream=%s",
		cfg.ServerPort, cfg.RateLimit, cfg.RateBurst, cfg.RedisAddr, cfg.RedisStreamName)

	return cfg
}

// getEnvOrDefault returns the env value or the provided fallback.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvAsInt parses an integer env var with a fallback default.
func getEnvAsInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("[config] warning: invalid int for %s=%q, using default %d", key, v, fallback)
		return fallback
	}
	return i
}
