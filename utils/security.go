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

	for _, host := range requestHostCandidates(r) {
		if strings.EqualFold(parsedOrigin.Host, host) {
			return true
		}
	}
	return false
}

func requestHostCandidates(r *http.Request) []string {
	hosts := make([]string, 0, 2)
	if h := strings.TrimSpace(r.Host); h != "" {
		hosts = append(hosts, h)
	}
	if h := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); h != "" {
		parts := strings.Split(h, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			hosts = append(hosts, strings.TrimSpace(parts[0]))
		}
	}
	return hosts
}
