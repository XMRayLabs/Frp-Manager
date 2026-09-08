package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/gin-gonic/gin"
)

type ApiService interface {
	Run()
	Stop()
}

type server struct {
	srv    *http.Server
	addr   net.Listener
	enable bool
}

var (
	_ ApiService = (*server)(nil)
)

func NewApiService(listen net.Listener, router *gin.Engine, enable bool) *server {
	return &server{
		srv: &http.Server{
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       2 * time.Minute,
			MaxHeaderBytes:    1 << 20,
		},
		addr:   listen,
		enable: enable,
	}
}

func (s *server) Run() {
	// 如果完全使用mux，可以不启动
	if !s.enable || s.addr == nil {
		return
	}
	if err := s.srv.Serve(s.addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Logger(context.Background()).WithError(err).Error("API server stopped unexpectedly")
	}
}

func (s *server) Stop() {
	if !s.enable || s.addr == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		_ = s.srv.Close()
	}
}
