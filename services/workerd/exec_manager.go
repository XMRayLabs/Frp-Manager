//go:build !windows

package workerd

import (
	"context"
	"os/exec"
	"syscall"
	"time"

	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/sirupsen/logrus"
)

type workerExecManager struct {
	//鐢ㄤ簬澶栧眰寰潖鐨勯€€鍑?
	signMap *utils.SyncMap[string, bool]
	//鐢ㄤ簬鎵цcancel鍑芥暟
	chanMap *utils.SyncMap[string, chan struct{}]
	// 鍙墽琛屾枃浠惰矾寰?
	binaryPath string
	// 榛樿鍙傛暟
	defaultArgs []string
}

// var ExecManager *execManager

func NewExecManager(binPath string, defaultArgs []string) app.WorkerExecManager {

	if len(defaultArgs) == 0 {
		defaultArgs = []string{"--watch", "--verbose"}
	}

	return &workerExecManager{
		signMap:     new(utils.SyncMap[string, bool]),
		chanMap:     new(utils.SyncMap[string, chan struct{}]),
		binaryPath:  binPath,
		defaultArgs: defaultArgs,
	}
}

func (m *workerExecManager) RunCmd(uid string, cwd string, argv []string) {
	ctx := context.Background()
	logger.Logger(context.Background()).Infof("start to run command, command id: [%s], argv: %s", uid, utils.MarshalForJson(argv))
	if _, ok := m.chanMap.Load(uid); ok {
		logger.Logger(ctx).Infof("command id: [%s] is already running, ignore", uid)
		return
	}

	c := make(chan struct{})
	m.chanMap.Store(uid, c)

	ctx, cancel := context.WithCancel(context.Background())

	go func(ctx context.Context, uid string, argv []string, m *workerExecManager) {
		defer func(uid string, m *workerExecManager) {
			m.signMap.Delete(uid)
		}(uid, m)

		logger.Logger(ctx).Infof("command id: [%s] is running!", uid)

		for {
			args := []string{}

			args = append(args, m.defaultArgs...)
			args = append(args, argv...)

			cmd := exec.CommandContext(ctx, m.binaryPath, args...)
			cmd.Dir = cwd
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: false}
			cmd.Stdout = logger.LoggerWriter("workerd", logrus.InfoLevel)
			cmd.Stderr = logger.LoggerWriter("workerd", logrus.ErrorLevel)
			if err := cmd.Run(); err != nil {
				logger.Logger(ctx).WithError(err).Errorf("command id: [%s] run failed, binary path: [%s], args: %s", uid, m.binaryPath, utils.MarshalForJson(args))
			}

			if exit, ok := m.signMap.Load(uid); ok && exit {
				return
			}
			time.Sleep(3 * time.Second)
		}
	}(ctx, uid, argv, m)

	go func(cancel context.CancelFunc, uid string, m *workerExecManager) {
		defer func(uid string, m *workerExecManager) {
			m.chanMap.Delete(uid)
		}(uid, m)

		if channel, ok := m.chanMap.Load(uid); ok {
			<-channel
			m.signMap.Store(uid, true)
			cancel()
			return
		} else {
			logger.Logger(ctx).Errorf("command id: [%s] is not running!", uid)
			return
		}
	}(cancel, uid, m)
}

func (m *workerExecManager) ExitCmd(uid string) {
	if channel, ok := m.chanMap.Load(uid); ok {
		channel <- struct{}{}
	}
}

func (m *workerExecManager) ExitAllCmd() {
	for uid := range m.chanMap.ToMap() {
		m.ExitCmd(uid)
	}
}

func (m *workerExecManager) UpdateBinaryPath(path string) {
	m.binaryPath = path
}
