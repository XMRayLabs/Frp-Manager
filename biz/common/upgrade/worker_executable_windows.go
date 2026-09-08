//go:build windows

package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const windowsWorkerPrefix = "frp-manager-upgrade-worker-"

func prepareWorkerExecutable(exePath, planPath string) (string, error) {
	workDir := filepath.Dir(planPath)
	if err := os.MkdirAll(workDir, 0700); err != nil {
		return "", fmt.Errorf("create Windows upgrade worker directory: %w", err)
	}

	staleWorkers, _ := filepath.Glob(filepath.Join(workDir, windowsWorkerPrefix+"*.exe"))
	for _, staleWorker := range staleWorkers {
		_ = os.Remove(staleWorker)
	}

	workerPath := filepath.Join(
		workDir,
		fmt.Sprintf("%s%d-%d.exe", windowsWorkerPrefix, os.Getpid(), time.Now().UnixNano()),
	)
	if err := copyFile(exePath, workerPath, 0700); err != nil {
		return "", fmt.Errorf("prepare Windows upgrade worker: %w", err)
	}
	return workerPath, nil
}

func scheduleWorkerCleanup() {
	executable, err := os.Executable()
	if err != nil || !strings.HasPrefix(filepath.Base(executable), windowsWorkerPrefix) {
		return
	}
	path, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return
	}
	_ = windows.MoveFileEx(path, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
}
