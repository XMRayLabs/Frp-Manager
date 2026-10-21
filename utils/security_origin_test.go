package utils

import (
	"net/http/httptest"
	"testing"
)

func TestWebSocketOriginCannotTrustForwardedHost(t *testing.T) {
	for _, tc := range []struct {
		origin, forwarded string
		allowed           bool
	}{
		{"https://panel.example.test", "", true},
		{"https://sibling.example.test", "sibling.example.test", false},
		{"http://panel.example.test", "", false},
		{"null", "", false},
		{"", "", true},
	} {
		r := httptest.NewRequest("GET", "https://panel.example.test/wsgrpc", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Forwarded-Host", tc.forwarded)
		if SameOriginWebSocketCheck(r) != tc.allowed {
			t.Errorf("unexpected origin result for %q", tc.origin)
		}
	}
}
