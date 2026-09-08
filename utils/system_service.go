package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/kardianos/service"
)

type SystemService struct {
	run func()
	service.Service
}

func (ss *SystemService) Start(s service.Service) error {
	go ss.iRun()
	return nil
}

func (ss *SystemService) Stop(s service.Service) error { return nil }

func (ss *SystemService) iRun() {
	defer func() {
		if service.Interactive() {
			ss.Stop(ss.Service)
		} else {
			ss.Service.Stop()
		}
	}()
	ss.run()
}

func CreateSystemService(svcName string, args []string, run func()) (service.Service, error) {
	return CreateSystemServiceWithOptions(svcName, args, run, nil)
}

func CreateSystemServiceWithOptions(svcName string, args []string, run func(), options service.KeyValue) (service.Service, error) {
	currentPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get current path failed, err: %v", err)
	}

	svcConfig := &service.Config{
		Name:             svcName,
		DisplayName:      "frp-manager",
		Description:      "this is frp-manager service, developed by [Sakurame1] - https://github.com/Sakurame1/frp-manager",
		Arguments:        args,
		WorkingDirectory: filepath.Dir(currentPath),
		EnvVars:          serviceEnvironment(currentPath),
		Option:           options,
	}

	ss := &SystemService{
		run: run,
	}

	s, err := service.New(ss, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("service New failed, err: %v", err)
	}
	return s, nil
}

func serviceEnvironment(executablePath string) map[string]string {
	if runtime.GOOS != "windows" {
		return nil
	}
	installDir := filepath.Dir(executablePath)
	return map[string]string{
		"FRP_MANAGER_ENV_FILE": filepath.Join(installDir, ".env"),
		"LOGGER_FILE":          filepath.Join(installDir, "service.log"),
	}
}

func ControlSystemService(svcName string, args []string, action string, run func()) error {
	return ControlSystemServiceWithOptions(svcName, args, action, run, nil)
}

func ControlSystemServiceWithOptions(svcName string, args []string, action string, run func(), options service.KeyValue) error {
	ctx := context.Background()

	logger.Logger(ctx).Info("try to ", action, " service, args:", redactServiceArgs(args))
	s, err := CreateSystemServiceWithOptions(svcName, args, run, options)
	if err != nil {
		logger.Logger(ctx).WithError(err).Error("create service controller failed")
		return err
	}

	if err := service.Control(s, action); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("controller %v service failed", action)
		return err
	}
	logger.Logger(ctx).Infof("controller %v service success", action)
	return nil
}

func redactServiceArgs(args []string) []string {
	redacted := append([]string(nil), args...)
	hideNext := false
	for i, arg := range redacted {
		if hideNext {
			redacted[i] = "[redacted]"
			hideNext = false
			continue
		}
		lower := strings.ToLower(arg)
		if lower == "-s" || lower == "--secret" || lower == "-j" || lower == "--join-token" {
			hideNext = true
			continue
		}
		if strings.HasPrefix(lower, "--join-token=") {
			redacted[i] = "--join-token=[redacted]"
		}
		if strings.HasPrefix(lower, "-j=") {
			redacted[i] = "-j=[redacted]"
		}
		if strings.HasPrefix(lower, "--secret=") {
			redacted[i] = "--secret=[redacted]"
		}
	}
	return redacted
}

func InstallToSystemPath(installPath string) error {
	currentPath, err := os.Executable()
	if err != nil {
		return err
	}

	targetPath := filepath.Join(installPath, filepath.Base(currentPath))

	src, err := os.Open(currentPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	err = os.Chmod(targetPath, 0755)
	if err != nil {
		return err
	}

	return nil
}
