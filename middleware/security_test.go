package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	for header, want := range map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
	} {
		got := recorder.Header().Get(header)
		if got == "" || (header == "Content-Security-Policy" && !strings.Contains(got, want)) || (header != "Content-Security-Policy" && got != want) {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(NewRateLimiter(1, time.Minute).Handler())
	router.POST("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/", nil))
	if first.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d", first.Code)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d", second.Code)
	}
}

func TestRateLimiterCapsUniqueClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(1, time.Minute)
	router := gin.New()
	router.Use(limiter.Handler())
	router.POST("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for i := 0; i < maxRateLimiterEntries; i++ {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.RemoteAddr = "192.0." + strconv.Itoa(i/256) + "." + strconv.Itoa(i%256) + ":1234"
		router.ServeHTTP(httptest.NewRecorder(), request)
	}

	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.RemoteAddr = "198.51.100.1:1234"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("overflow request status = %d", recorder.Code)
	}
	if len(limiter.entries) != maxRateLimiterEntries {
		t.Fatalf("rate limiter entries = %d", len(limiter.entries))
	}
}
