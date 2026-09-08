//go:build windows

package upgrade

import (
	"fmt"
	"runtime"
)

func detectAssetName(clientOnly bool) (string, error) {
	prefix := "frp-manager"
	if clientOnly {
		prefix = "frp-manager-client"
	}
	switch runtime.GOARCH {
	case "amd64":
		return prefix + "-windows-amd64.exe", nil
	case "arm64":
		return prefix + "-windows-arm64.exe", nil
	default:
		return "", fmt.Errorf("鏆備笉鏀寔鐨勭郴缁?鏋舵瀯: %s %s", runtime.GOOS, runtime.GOARCH)
	}
}
