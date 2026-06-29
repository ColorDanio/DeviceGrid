package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple in-memory token-bucket per key (IP + endpoint).
// Suitable for protecting auth endpoints against brute force.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64 // max bucket size
	cleanup time.Duration
}

type bucket struct {
	tokens   float64
	lastFill time.Time
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   float64(burst),
		cleanup: 10 * time.Minute,
	}
	go rl.janitor()
	return rl
}

func (rl *RateLimiter) janitor() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.cleanup)
		for k, b := range rl.buckets {
			if b.lastFill.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: rl.burst, lastFill: now}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.burst {
		b.tokens = rl.burst
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware returns a gin middleware that rate-limits by client IP + path.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		if !rl.allow(key) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, APIResponse{
				Code:    429,
				Message: "too many requests, slow down",
			})
			return
		}
		c.Next()
	}
}

// RateLimit returns a gin middleware that allows `count` requests per IP+path per `window`.
// Each IP+path combination gets its own token bucket. Implemented as a simple
// in-memory limiter suitable for protecting auth endpoints against brute force.
func RateLimit(count int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(float64(count)/window.Seconds(), count)
	return limiter.Middleware()
}
