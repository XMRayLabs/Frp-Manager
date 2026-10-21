package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NormalizeAllowedPorts accepts a single inclusive range or a single port.
// Empty means unrestricted and preserves all pre-existing authorizations.
func NormalizeAllowedPorts(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return "", fmt.Errorf("端口格式应为 62000-63999 或单个端口，留空表示不限制")
	}
	ports := make([]int, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		for _, c := range part {
			if c < '0' || c > '9' {
				return "", fmt.Errorf("端口必须是 1-65535 的整数")
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("端口必须是 1-65535 的整数")
		}
		ports[i] = n
	}
	if len(ports) == 1 {
		return strconv.Itoa(ports[0]), nil
	}
	if ports[0] > ports[1] {
		return "", fmt.Errorf("起始端口不能大于结束端口")
	}
	return fmt.Sprintf("%d-%d", ports[0], ports[1]), nil
}

func ValidateProxyAllowedPorts(allowed string, content, serverContent []byte) error {
	normalized, err := NormalizeAllowedPorts(allowed)
	if err != nil {
		return err
	}
	if normalized == "" {
		return nil
	}
	var proxy struct {
		Type       string `json:"type"`
		RemotePort int    `json:"remotePort"`
	}
	if err := json.Unmarshal(content, &proxy); err != nil {
		return fmt.Errorf("隧道配置无效，无法检查授权端口")
	}
	port := proxy.RemotePort
	switch proxy.Type {
	case "tcp", "udp":
	case "http", "https", "tcpmux":
		var server struct {
			HTTP   int `json:"vhostHTTPPort"`
			HTTPS  int `json:"vhostHTTPSPort"`
			TCPMux int `json:"tcpmuxHTTPConnectPort"`
		}
		if err := json.Unmarshal(serverContent, &server); err != nil {
			return fmt.Errorf("服务端配置无效，无法检查授权端口")
		}
		switch proxy.Type {
		case "http":
			port = server.HTTP
		case "https":
			port = server.HTTPS
		case "tcpmux":
			port = server.TCPMux
		}
	case "stcp", "sudp", "xtcp":
		return nil // Visitor-based tunnels do not allocate a public server listening port.
	default:
		return fmt.Errorf("无法检查协议 %s 的授权端口", proxy.Type)
	}
	bounds := strings.Split(normalized, "-")
	start, _ := strconv.Atoi(bounds[0])
	end := start
	if len(bounds) == 2 {
		end, _ = strconv.Atoi(bounds[1])
	}
	if port == 0 {
		return fmt.Errorf("已限制服务端端口为 %s，不能使用未指定或随机端口", normalized)
	}
	if port < start || port > end {
		return fmt.Errorf("服务端端口 %d 超出授权范围 %s", port, normalized)
	}
	return nil
}
