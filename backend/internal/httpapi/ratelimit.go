package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ipRateLimiter is a simple in-memory per-IP token-bucket limiter — enough to
// blunt credential-stuffing/registration-spam against a single-process
// deployment (plan/mcp.md's OAuth surface has no confidential client secret,
// so /oauth/register, /oauth/token, and /oauth/authorize are the exposed
// abuse surface). Not a substitute for a real edge/WAF rate limiter in a
// multi-replica deployment — see Task 17's production-hardening notes. The
// bucket map is never swept: bounded in practice by the number of distinct
// IPs that ever hit these three low-traffic auth routes over the process's
// lifetime, not a real unbounded-growth risk at this scope.
type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // tokens added per second
	burst   float64 // bucket capacity (and max tokens)
}

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// newIPRateLimiter builds a limiter allowing burst immediate requests, then
// refilling at ratePerSecond.
func newIPRateLimiter(ratePerSecond, burst float64) *ipRateLimiter {
	return &ipRateLimiter{buckets: map[string]*tokenBucket{}, rate: ratePerSecond, burst: burst}
}

func (l *ipRateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.lastSeen).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.lastSeen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateLimit is Gin middleware enforcing l against the request's client IP.
func rateLimit(l *ipRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c.ClientIP()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too_many_requests", "error_description": "rate limit exceeded — slow down and retry shortly"})
			return
		}
		c.Next()
	}
}
