package server

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/metrics/mem"
	"github.com/fatedier/frp/pkg/policy/security"
	"github.com/fatedier/frp/server"
	"github.com/sourcegraph/conc"
)

type serverImpl struct {
	srv       *server.Service
	Common    *v1.ServerConfig
	firstSync sync.Once
	running   atomic.Bool
}

func NewServerHandler(svrCfg *v1.ServerConfig) app.ServerHandler {
	if err := svrCfg.Complete(); err != nil {
		logger.Logger(context.Background()).Panic(err)
	}
	ctx := context.Background()

	validator := validation.NewConfigValidator(security.NewUnsafeFeatures(nil))
	warning, err := validator.ValidateServerConfig(svrCfg)
	if warning != nil {
		logger.Logger(ctx).WithError(err).Warnf("validate server config warning: %+v", warning)
	}
	if err != nil {
		logger.Logger(ctx).Panic(err)
	}

	var svr *server.Service

	if svr, err = server.NewService(svrCfg); err != nil {
		logger.Logger(ctx).WithError(err).Panic("cannot create server, exit and restart")
	}

	logger.Logger(ctx).Debugf("create server, config is: [ %+v ]", svrCfg)

	return &serverImpl{
		srv:       svr,
		Common:    svrCfg,
		firstSync: sync.Once{},
	}
}

func (s *serverImpl) Run() {
	s.running.Store(true)
	defer s.running.Store(false)
	wg := conc.NewWaitGroup()
	wg.Go(func() { s.srv.Run(context.Background()) })
	wg.Wait()
}

func (s *serverImpl) Stop() {
	s.running.Store(false)
	c := context.Background()
	wg := conc.NewWaitGroup()
	wg.Go(func() {
		err := s.srv.Close()
		if err != nil {
			logger.Logger(c).Errorf("close server error: %v", err)
		}
		logger.Logger(c).Infof("server closed")
	})
	wg.Wait()
}

func (s *serverImpl) GetCommonCfg() *v1.ServerConfig {
	return s.Common
}

func (s *serverImpl) GetMem() *mem.ServerStats {
	return mem.StatsCollector.GetServer()
}

func (s *serverImpl) GetProxyStatsByType(proxyType v1.ProxyType) []*mem.ProxyStats {
	return mem.StatsCollector.GetProxiesByType(string(proxyType))
}

func (s *serverImpl) IsFirstSync() bool {
	result := false
	s.firstSync.Do(func() {
		result = true
	})
	return result
}

// Running reports whether the FRPS service loop is active.
func (s *serverImpl) Running() bool { return s.running.Load() }
