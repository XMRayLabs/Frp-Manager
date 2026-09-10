package client

import (
	"strings"
	"testing"
)

func TestSharedConfigRemovesDeviceCredentials(t *testing.T) {
	safe := sharedClientConfig(`{"serverAddr":"example","serverPort":7000,"auth":{"token":"device-auth"},"metadatas":{"token":"user-token"},"transport":{"tls":{"keyFile":"private-path"}},"proxies":[{"name":"p","plugin":{"type":"socks5","password":"business-password"}}]}`)
	if safe == nil {
		t.Fatal("shared config unavailable")
	}
	for _, secret := range []string{"device-auth", "user-token", "private-path"} {
		if strings.Contains(*safe, secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	if !strings.Contains(*safe, "business-password") {
		t.Fatal("shared tunnel business configuration missing")
	}
}
