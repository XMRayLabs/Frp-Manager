package master

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/Sakurame1/frp-manager/biz/master/client"
	masterserver "github.com/Sakurame1/frp-manager/biz/master/server"
	"github.com/Sakurame1/frp-manager/biz/master/shell"
	"github.com/Sakurame1/frp-manager/biz/master/streamlog"
	"github.com/Sakurame1/frp-manager/biz/master/wg"
	"github.com/Sakurame1/frp-manager/biz/master/worker"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"
)

type server struct {
	pb.UnimplementedMasterServer
	appInstance app.Application
}

func (s *server) ReportWireGuardRuntimeInfo(ctx context.Context, req *pb.ReportWireGuardRuntimeInfoReq) (*pb.ReportWireGuardRuntimeInfoResp, error) {
	logger.Logger(ctx).Debugf("report wireguard runtime info, clientID: [%s], interfaceName: [%s]", req.GetBase().GetClientId(), req.GetInterfaceName())
	appCtx := app.NewContext(ctx, s.appInstance)

	if client, err := client.ValidateClientRequest(appCtx, req.GetBase()); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot validate client request")
		return nil, err
	} else if client.Stopped {
		logger.Logger(appCtx).Infof("report wireguard runtime info, client [%s] is stopped", req.GetBase().GetClientId())
		return &pb.ReportWireGuardRuntimeInfoResp{
			Status: &pb.Status{
				Code:    pb.RespCode_RESP_CODE_NOT_FOUND,
				Message: "client stopped",
			},
		}, nil
	}
	logger.Logger(appCtx).Debugf("validate client success, clientID: [%s], interfaceName: [%s]", req.GetBase().GetClientId(), req.GetInterfaceName())

	return wg.ReportWireGuardRuntimeInfo(appCtx, req)
}

// ListClientWireGuards implements pb.MasterServer.
func (s *server) ListClientWireGuards(ctx context.Context, req *pb.ListClientWireGuardsRequest) (*pb.ListClientWireGuardsResponse, error) {
	logger.Logger(ctx).Debugf("list client wire guards, clientID: [%s]", req.GetBase().GetClientId())
	appCtx := app.NewContext(ctx, s.appInstance)

	if client, err := client.ValidateClientRequest(appCtx, req.GetBase()); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot validate client request")
		return nil, err
	} else if client.Stopped {
		logger.Logger(appCtx).Infof("list client wire guards, client [%s] is stopped", req.GetBase().GetClientId())
		return &pb.ListClientWireGuardsResponse{
			Status: &pb.Status{
				Code:    pb.RespCode_RESP_CODE_NOT_FOUND,
				Message: "client stopped",
			},
		}, nil
	}

	resp, err := wg.ListClientWireGuards(appCtx, req)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot list client wire guards")
		return nil, err
	}

	return resp, nil
}

// ListClientWorkers implements pb.MasterServer.
func (s *server) ListClientWorkers(ctx context.Context, req *pb.ListClientWorkersRequest) (*pb.ListClientWorkersResponse, error) {
	logger.Logger(ctx).Debugf("list client workers, clientID: [%s]", req.GetBase().GetClientId())
	appCtx := app.NewContext(ctx, s.appInstance)

	if client, err := client.ValidateClientRequest(appCtx, req.GetBase()); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot validate client request")
		return nil, err
	} else if client.Stopped {
		logger.Logger(appCtx).Infof("list client workers, client [%s] is stopped", req.GetBase().GetClientId())
		return &pb.ListClientWorkersResponse{
			Status: &pb.Status{
				Code:    pb.RespCode_RESP_CODE_NOT_FOUND,
				Message: "client stopped",
			},
		}, nil
	}

	logger.Logger(appCtx).Debugf("validate client success, clientID: [%s]", req.GetBase().GetClientId())

	return worker.ListClientWorkers(appCtx, req)
}

func newRpcServer(appInstance app.Application, creds credentials.TransportCredentials) *grpc.Server {
	s := grpc.NewServer(
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(16<<20),
		grpc.MaxSendMsgSize(16<<20),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 30 * time.Minute,
			Time:              2 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	pb.RegisterMasterServer(s, &server{
		appInstance: appInstance,
	})
	return s
}

func runRpcServer(appInstance app.Application, s *grpc.Server) {
	ctx := context.Background()

	lis, err := net.Listen("tcp", conf.RPCListenAddr(appInstance.GetConfig()))
	if err != nil {
		logger.Logger(ctx).Fatalf("rpc server failed to listen: %v", err)
	}

	logger.Logger(ctx).Infof("start server")
	if err := s.Serve(lis); err != nil {
		logger.Logger(ctx).Fatalf("failed to serve: %v", err)
	}
}

// PullClientConfig implements pb.MasterServer.
func (s *server) PullClientConfig(ctx context.Context, req *pb.PullClientConfigReq) (*pb.PullClientConfigResp, error) {
	logger.Logger(ctx).Debugf("pull client config, clientID: [%+v]", req.GetBase().GetClientId())
	return client.RPCPullConfig(app.NewContext(ctx, s.appInstance), req)
}

// PullServerConfig implements pb.MasterServer.
func (s *server) PullServerConfig(ctx context.Context, req *pb.PullServerConfigReq) (*pb.PullServerConfigResp, error) {
	logger.Logger(ctx).Debugf("pull server config, serverID: [%+v]", req.GetBase().GetServerId())
	return masterserver.RPCPullConfig(app.NewContext(ctx, s.appInstance), req)
}

// FRPCAuth implements pb.MasterServer.
func (s *server) FRPCAuth(ctx context.Context, req *pb.FRPAuthRequest) (*pb.FRPAuthResponse, error) {
	logger.Logger(ctx).Debugf("frpc auth, user: [%+v] ,serverID: [%+v]", req.GetUser(), req.GetBase().GetServerId())
	return masterserver.FRPAuth(app.NewContext(ctx, s.appInstance), req)
}

// ServerSend implements pb.MasterServer.
func (s *server) ServerSend(sender pb.Master_ServerSendServer) error {
	ctx := app.NewContext(context.Background(), s.appInstance)

	logger.Logger(ctx).Infof("server get a client connected")
	var done chan bool
	var connectionDone <-chan struct{}
	for {
		req, err := sender.Recv()
		if err == io.EOF {
			logger.Logger(ctx).Infof("finish server send, client id: [%s]", "closed")
			return nil
		}

		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot recv from client, id: [%s]", "unknown")
			return err
		}

		cliType := ""

		if req.GetEvent() == pb.Event_EVENT_REGISTER_CLIENT || req.GetEvent() == pb.Event_EVENT_REGISTER_SERVER {
			var registeredID string
			connector, err := func() (*defs.Connector, error) {
				models.NodeIdentityMu.Lock()
				defer models.NodeIdentityMu.Unlock()
				if len(req.GetSecret()) == 0 {
					logger.Logger(ctx).Errorf("rpc auth token is empty")
					return nil, fmt.Errorf("rpc auth token is invalid")
				}
				var secret string
				canonicalID := req.GetClientId()
				switch req.GetEvent() {
				case pb.Event_EVENT_REGISTER_CLIENT:
					cli, err := dao.NewQuery(ctx).ValidateClientSecret(req.GetClientId(), req.GetSecret())
					if err != nil {
						logger.Logger(context.Background()).WithError(err).Errorf("cannot get client, %s id: [%s]", req.GetEvent().String(), req.GetClientId())
						return nil, err
					}
					canonicalID = cli.ClientID
					if cli.OriginClientID != "" {
						canonicalID = cli.OriginClientID
					}
					secret = cli.ConnectSecret
					cliType = defs.CliTypeClient
				case pb.Event_EVENT_REGISTER_SERVER:
					srv, err := dao.NewQuery(ctx).ValidateServerSecret(req.GetClientId(), req.GetSecret())
					if err != nil {
						logger.Logger(context.Background()).WithError(err).Errorf("cannot get server, %s id: [%s]", req.GetEvent().String(), req.GetClientId())
						return nil, err
					}
					canonicalID = srv.ServerID
					secret = srv.ConnectSecret
					cliType = defs.CliTypeServer
				}

				if !utils.SecureStringEqual(secret, req.GetSecret()) {
					logger.Logger(ctx).Errorf("invalid secret, %s id: [%s]", req.GetEvent().String(), req.GetClientId())
					return nil, fmt.Errorf("invalid secret, %s id: [%s]", req.GetEvent().String(), req.GetClientId())
				}

				if cliType == defs.CliTypeClient {
					if err := dao.NewMutation(ctx).AdminUpdateClientLastSeen(canonicalID); err != nil {
						logger.Logger(ctx).Errorf("cannot update client last seen, %s id: [%s]", req.GetEvent().String(), req.GetClientId())
					}
				}

				var clientVersion *pb.ClientVersion
				if len(req.GetData()) > 0 {
					clientVersion = &pb.ClientVersion{}
					if err := proto.Unmarshal(req.GetData(), clientVersion); err != nil {
						clientVersion = nil
					}
				}

				connector := s.appInstance.GetClientsManager().Set(canonicalID, cliType, &rpc.IdentityStream{Master_ServerSendServer: sender, ID: req.GetClientId()}, clientVersion)
				connector.SendMu.Lock()
				done = rpc.RecvConnector(s.appInstance, canonicalID, connector)
				connectionDone = connector.Done
				registeredID = canonicalID
				logger.Logger(ctx).Infof("register success, client id: [%s], client type: [%s]", req.GetClientId(), cliType)
				return connector, nil
			}()
			if err != nil {
				return err
			}
			defer s.appInstance.GetClientsManager().RemoveIfCurrent(registeredID, connector)
			err = sender.Send(&pb.ServerMessage{Event: req.GetEvent(), ClientId: registeredID, SessionId: registeredID})
			connector.SendMu.Unlock()
			if err != nil {
				return err
			}
			break
		}
	}
	select {
	case <-done:
	case <-connectionDone:
	case <-sender.Context().Done():
	}
	return nil
}

// PushProxyInfo implements pb.MasterServer.
func (s *server) PushProxyInfo(ctx context.Context, req *pb.PushProxyInfoReq) (*pb.PushProxyInfoResp, error) {
	logger.Logger(ctx).Debugf("push proxy info, req server: [%+v]", req.GetProxyInfos())
	return masterserver.PushProxyInfo(app.NewContext(ctx, s.appInstance), req)
}

func (s *server) PushClientStreamLog(sender pb.Master_PushClientStreamLogServer) error {
	return streamlog.PushClientStreamLog(app.NewContext(context.Background(), s.appInstance), sender)
}

func (s *server) PushServerStreamLog(sender pb.Master_PushServerStreamLogServer) error {
	return streamlog.PushServerStreamLog(app.NewContext(context.Background(), s.appInstance), sender)
}

func (s *server) PTYConnect(sender pb.Master_PTYConnectServer) error {
	return shell.PTYConnect(app.NewContext(context.Background(), s.appInstance), sender)
}
