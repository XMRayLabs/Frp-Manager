//go:build !windows

package upgrade

func prepareWorkerExecutable(exePath, _ string) (string, error) {
	return exePath, nil
}

func scheduleWorkerCleanup() {}
