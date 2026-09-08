//go:build !windows

package upgrade

import (
	"fmt"
	"runtime"
)

func detectAssetName(clientOnly bool) (string, error) {
	osName := runtime.GOOS
	machine := unameMachine()
	prefix := "frp-manager"
	if clientOnly {
		prefix = "frp-manager-client"
	}
	if len(machine) == 0 {
		// fallback
		machine = runtime.GOARCH
	}

	switch osName {
	case "linux":
		switch machine {
		case "x86_64", "amd64":
			return prefix + "-linux-amd64", nil
		case "aarch64", "arm64":
			return prefix + "-linux-arm64", nil
		case "armv7l":
			return prefix + "-linux-armv7l", nil
		case "armv6l":
			return prefix + "-linux-armv6l", nil
		}
	case "darwin":
		switch machine {
		case "x86_64", "amd64":
			return prefix + "-darwin-amd64", nil
		case "arm64":
			return prefix + "-darwin-arm64", nil
		}
	}
	return "", fmt.Errorf("鏆備笉鏀寔鐨勭郴缁?鏋舵瀯: %s %s", osName, machine)
}
