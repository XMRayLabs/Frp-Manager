package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/kardianos/service"
)

type AutoOptions struct {
	CurrentVersion string
	ClientOnly     bool
	HTTPProxy      string
	ServiceArgs    []string
}

// AutoUpdateOnStartup checks GitHub Releases and upgrades before the node starts.
// The returned bool tells the caller to stop while a worker replaces the binary.
func AutoUpdateOnStartup(ctx context.Context, auto AutoOptions) (bool, error) {
	if runtime.GOOS == "android" {
		logger.Logger(ctx).Info("auto update: Android core is managed by the signed APK package")
		return false, nil
	}
	if runtime.GOOS == "darwin" && service.Interactive() {
		executable, _ := os.Executable()
		if isMacAppBundlePath(executable) {
			logger.Logger(ctx).Info("auto update: bundled macOS core is managed with the app package; skipping in-place replacement")
			return false, nil
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	release, available, err := CheckForUpdate(checkCtx, auto.CurrentVersion, auto.ClientOnly, auto.HTTPProxy)
	if err != nil {
		cancel()
		return false, err
	}
	cancel()
	if !available {
		logger.Logger(ctx).Infof("auto update: current version %s is up to date", auto.CurrentVersion)
		return false, nil
	}

	interactive := service.Interactive()
	opts := Options{
		Version:        release.TagName,
		HTTPProxy:      auto.HTTPProxy,
		Backup:         true,
		ClientOnly:     auto.ClientOnly,
		RestartService: !interactive,
		ServiceArgs:    auto.ServiceArgs,
	}
	if !interactive {
		opts.ServiceName = "frpp"
	}
	if interactive && runtime.GOOS == "windows" {
		opts.Relaunch = true
		opts.RelaunchArgs = append([]string(nil), os.Args[1:]...)
	}

	logger.Logger(ctx).Infof("auto update: upgrading from %s to %s", auto.CurrentVersion, release.Version)
	downloadCtx, cancelDownload := context.WithTimeout(ctx, 3*time.Minute)
	defer cancelDownload()
	result, err := StartWithResult(downloadCtx, opts)
	if err != nil {
		return false, err
	}
	if result.Dispatched {
		logger.Logger(ctx).Info("auto update: replacement worker dispatched; stopping current startup")
		return true, nil
	}

	if interactive {
		target, err := resolveTargetPath("")
		if err != nil {
			return false, err
		}
		logger.Logger(ctx).Info("auto update: relaunching the updated binary")
		if err := relaunchCurrent(target, os.Args[1:]); err != nil {
			return false, fmt.Errorf("relaunch updated binary: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func isMacAppBundlePath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(normalized, ".app/contents/")
}
