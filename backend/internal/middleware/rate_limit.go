package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/pkg/response"
)

// rateBucket tracks attempt count and the timestamp when it resets.
type rateBucket struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

// allow returns true if the bucket has capacity for one more attempt,
// false when the per-window limit is exhausted.
func (b *rateBucket) allow(limit int, window time.Duration) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.After(b.resetAt) {
		// Window has expired — start a fresh bucket.
		b.count = 0
		b.resetAt = now.Add(window)
	}

	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

// loginRateLimiter holds the per-IP state map and configuration.
type loginRateLimiter struct {
	buckets sync.Map // map[string]*rateBucket
	limit   int
	window  time.Duration
}

// check resolves the bucket for ip and delegates to allow().
func (l *loginRateLimiter) check(ip string) bool {
	val, _ := l.buckets.LoadOrStore(ip, &rateBucket{
		resetAt: time.Now().Add(l.window),
	})
	return val.(*rateBucket).allow(l.limit, l.window)
}

// LoginRateLimit returns a Gin middleware that caps POST /api/auth/login
// to limit attempts per window per client IP. Responds 429 when exceeded.
// Uses an in-process sync.Map — no external dependencies required.
func LoginRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &loginRateLimiter{
		limit:  limit,
		window: window,
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.check(ip) {
			response.Error(c, http.StatusTooManyRequests, "too many login attempts, please try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}
