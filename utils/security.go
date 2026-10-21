package utils

import (
	crand "crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

func SecureRandomString(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	buf := make([]byte, byteLength)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func SameOriginWebSocketCheck(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil || parsedOrigin.Host == "" {
		return false
	}

	if parsedOrigin.User != nil || (parsedOrigin.Scheme != "https" && parsedOrigin.Scheme != "http") {
		return false
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return strings.EqualFold(parsedOrigin.Host, r.Host) && parsedOrigin.Scheme == scheme
}
