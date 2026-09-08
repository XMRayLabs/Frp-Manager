package upgrade

import (
	"os"
	"path/filepath"
	"runtime"
)

func defaultWorkDir() string {
	// 瀵逛簬 systemd 鍦烘櫙锛屼紭鍏堜娇鐢ㄦ寔涔呯洰褰曪紝鏂逛究 upgrader service 璇诲彇 plan/status
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		return filepath.Join(string(os.PathSeparator), "etc", "frpp", "upgrade")
	}
	return filepath.Join(os.TempDir(), "frp-manager-upgrade")
}
