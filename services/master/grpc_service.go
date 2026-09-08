package master

import (
	"time"

	"github.com/Sakurame1/frp-manager/services/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type MasterService interface {
	Run()
	Stop()
	GetServer() *grpc.Server
}

type master struct {
	grpcServer  *grpc.Server
	appInstance app.Application
}

func NewMasterService(appInstance app.Application, creds credentials.TransportCredentials) MasterService {
	s := newRpcServer(appInstance, creds)
	return &master{
		grpcServer:  s,
		appInstance: appInstance,
	}
}

func (s *master) Run() {
	runRpcServer(s.appInstance, s.grpcServer)
}

func (s *master) Stop() {
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		s.grpcServer.Stop()
	}
}

func (s *master) GetServer() *grpc.Server {
	return s.grpcServer
}
