package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/http/response"
)

type clientBucket struct {
	tokens     int
	lastRefill time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	capacity int
	refill   time.Duration
	clients  map[string]*clientBucket
}

func NewRateLimiter(capacity int, refill time.Duration) *RateLimiter {
	rl := &RateLimiter{
		capacity: capacity,
		refill:   refill,
		clients:  make(map[string]*clientBucket),
	}
	// Background cleanup of stale IP buckets every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for ip, b := range rl.clients {
				if b.lastRefill.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		rl.mu.Lock()
		b, exists := rl.clients[ip]
		now := time.Now()
		if !exists {
			b = &clientBucket{tokens: rl.capacity - 1, lastRefill: now}
			rl.clients[ip] = b
			rl.mu.Unlock()
			c.Next()
			return
		}

		elapsed := now.Sub(b.lastRefill)
		tokensToAdd := int(elapsed / rl.refill)
		if tokensToAdd > 0 {
			b.tokens = min(rl.capacity, b.tokens+tokensToAdd)
			b.lastRefill = now
		}

		if b.tokens <= 0 {
			rl.mu.Unlock()
			c.Header("Retry-After", "60")
			response.Error(c, http.StatusTooManyRequests, "too many requests; please slow down")
			c.Abort()
			return
		}

		b.tokens--
		rl.mu.Unlock()
		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
