package utils

import (
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const runtimeDNSServersEnv = "FRP_MANAGER_DNS_SERVERS"

// ConfigureRuntimeDNSFromEnv lets embedded clients supply DNS servers from the
// host platform when its resolver configuration is not visible to Go.
func ConfigureRuntimeDNSFromEnv() int {
	servers := runtimeDNSServerAddresses(os.Getenv(runtimeDNSServersEnv))
	if len(servers) == 0 {
		return 0
	}

	var next atomic.Uint64
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			index := next.Add(1) - 1
			dialer := net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, servers[index%uint64(len(servers))])
		},
	}
	return len(servers)
}

func runtimeDNSServerAddresses(raw string) []string {
	var servers []string
	seen := make(map[string]struct{})
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		address := value
		if _, _, err := net.SplitHostPort(value); err != nil {
			address = net.JoinHostPort(value, "53")
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		servers = append(servers, address)
	}
	return servers
}
