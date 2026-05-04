package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter returns a Gin middleware that enforces a global token-bucket rate limit.
//
// The token bucket algorithm allows short bursts up to `burst` while sustaining
// a long-term average of `rps` requests per second. Requests that cannot acquire
// a token are immediately rejected with 429 — no queueing, no blocking.
func RateLimiter(rps int, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"message": "server is under heavy load, please retry later",
			})
			return
		}
		c.Next()
	}
}
