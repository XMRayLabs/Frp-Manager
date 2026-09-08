package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxRequestBodyBytes   int64 = 8 << 20
	maxRateLimiterEntries       = 4096
)

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")

		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/auth" {
			headers.Set("Cache-Control", "no-store")
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		}
		c.Next()
	}
}

type rateLimitEntry struct {
	count int
	reset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateLimitEntry
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
	}
}

func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()

		r.mu.Lock()
		entry, ok := r.entries[key]
		if !ok && len(r.entries) >= maxRateLimiterEntries {
			for entryKey, candidate := range r.entries {
				if now.After(candidate.reset) {
					delete(r.entries, entryKey)
				}
			}
		}
		if !ok && len(r.entries) >= maxRateLimiterEntries {
			r.mu.Unlock()
			c.Header("Retry-After", strconv.Itoa(max(1, int(r.window.Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limiter capacity reached"})
			return
		}
		if !ok || now.After(entry.reset) {
			entry = rateLimitEntry{reset: now.Add(r.window)}
		}
		entry.count++
		r.entries[key] = entry
		allowed := entry.count <= r.limit
		retryAfter := max(1, int(time.Until(entry.reset).Seconds()))
		r.mu.Unlock()

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}
