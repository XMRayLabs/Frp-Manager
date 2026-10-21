package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Browser cookies must not authorize a request initiated by a different origin,
// including a sibling subdomain (SameSite cookies alone do not prevent this).
func checkBrowserOrigin(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	raw := c.GetHeader("Origin")
	if raw == "" {
		raw = c.GetHeader("Referer")
	}
	if raw == "" {
		site := c.GetHeader("Sec-Fetch-Site")
		return site == "" || site == "same-origin" || site == "none"
	}
	origin, err := url.Parse(raw)
	if err != nil || origin.User != nil || origin.Host == "" || (origin.Scheme != "https" && origin.Scheme != "http") {
		return false
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return strings.EqualFold(origin.Host, c.Request.Host) && origin.Scheme == scheme
}
