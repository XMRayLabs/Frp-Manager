package rpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/Sakurame1/frp-manager/utils/wsgrpc"
	"github.com/imroc/req/v3"
	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type masterClient struct {
	cli         pb.MasterClient
	inited      bool
	appInstance app.Application
}

func (m *masterClient) Call() pb.MasterClient {
	if !m.inited {
		m.cli = newMasterCli(m.appInstance)
		m.inited = true
	}
	return m.cli
}

func NewMasterCli(appInstance app.Application) *masterClient {
	logger.Logger(context.Background()).Debugf("creating new master client")
	return &masterClient{
		inited:      false,
		appInstance: appInstance,
	}
}

func newMasterCli(appInstance app.Application) pb.MasterClient {
	connInfo := conf.GetRPCConnInfo(appInstance.GetConfig())
	ctx := context.Background()

	opt := []grpc.DialOption{}

	switch connInfo.Scheme {
	case conf.GRPC:
		if appInstance.GetConfig().Client.TLSRpc {
			logger.Logger(ctx).Infof("use tls rpc")
			opt = append(opt, grpc.WithTransportCredentials(appInstance.GetRPCCred()))
		} else {
			logger.Logger(ctx).Infof("use insecure rpc")
			opt = append(opt, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	case conf.WS, conf.WSS:
		logger.Logger(ctx).Infof("use ws/wss rpc")

		wsURL := fmt.Sprintf("%s://%s/wsgrpc", connInfo.Scheme, connInfo.Host)
		header := http.Header{}
		wsDialer := wsgrpc.WebsocketDialer(wsURL,
			header,
			appInstance.GetConfig().Client.TLSInsecureSkipVerify,
			logger.Logger(ctx),
		)
		opt = append(opt, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(wsDialer))
	}

	logger.Logger(ctx).Debugf("creating new grpc client to [%s]", utils.MarshalForJson(connInfo))
	conn, err := grpc.NewClient(connInfo.Host, opt...)

	if err != nil {
		logger.Logger(ctx).Fatalf("did not connect: %v", err)
	}

	logger.Logger(ctx).Debugf("grpc client created")

	return pb.NewMasterClient(conn)
}

func httpCli(cfg conf.Config) *req.Client {
	c := req.C().SetTimeout(15 * time.Second)
	if cfg.Client.TLSInsecureSkipVerify {
		c.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return c
}

func GetClientCert(appInstance app.Application, clientID, clientSecret string, clientType pb.ClientType) ([]byte, error) {
	apiEndpoint := conf.GetAPIURL(appInstance.GetConfig())
	c := httpCli(appInstance.GetConfig())

	rawReq, err := proto.Marshal(&pb.GetClientCertRequest{
		ClientId:     clientID,
		ClientSecret: clientSecret,
		ClientType:   clientType,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal client certificate request: %w", err)
	}
	r, err := c.R().SetHeader("Content-Type", "application/x-protobuf").
		SetBodyBytes(rawReq).Post(apiEndpoint + "/api/v1/auth/cert")
	if err != nil {
		return nil, fmt.Errorf("request client certificate: %w", err)
	}
	if !r.IsSuccessState() {
		return nil, fmt.Errorf("request client certificate returned HTTP %d", r.GetStatusCode())
	}

	resp := &pb.GetClientCertResponse{}
	err = proto.Unmarshal(r.Bytes(), resp)
	if err != nil {
		return nil, fmt.Errorf("decode client certificate response: %w", err)
	}
	if resp.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
		return nil, fmt.Errorf("request client certificate rejected: %s", resp.GetStatus().GetMessage())
	}
	if len(resp.GetCert()) == 0 {
		return nil, errors.New("request client certificate returned an empty certificate")
	}
	return resp.GetCert(), nil
}

func InitClient(cfg conf.Config, clientID, joinToken string, ephemeral *bool) (*pb.InitClientResponse, error) {
	apiEndpoint := conf.GetAPIURL(cfg)

	c := httpCli(cfg)

	if ephemeral == nil {
		ephemeral = lo.ToPtr(false) // persistent nodes by default
	}

	rawReq, err := proto.Marshal(&pb.InitClientRequest{
		ClientId:  &clientID,
		Ephemeral: ephemeral,
	})
	if err != nil {
		return nil, err
	}

	r, err := c.R().SetHeader("Content-Type", "application/x-protobuf").
		SetHeader(defs.AuthorizationKey, joinToken).
		SetBodyBytes(rawReq).Post(apiEndpoint + "/api/v1/client/init")
	if err != nil {
		return nil, err
	}

	resp := &pb.InitClientResponse{}
	err = proto.Unmarshal(r.Bytes(), resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func GetClient(cfg conf.Config, clientID, joinToken string) (*pb.GetClientResponse, error) {
	apiEndpoint := conf.GetAPIURL(cfg)
	c := httpCli(cfg)

	rawReq, err := proto.Marshal(&pb.GetClientRequest{
		ClientId: &clientID,
	})
	if err != nil {
		return nil, err
	}

	r, err := c.R().SetHeader("Content-Type", "application/x-protobuf").
		SetHeader(defs.AuthorizationKey, joinToken).
		SetBodyBytes(rawReq).Post(apiEndpoint + "/api/v1/client/get")
	if err != nil {
		return nil, err
	}

	resp := &pb.GetClientResponse{}
	err = proto.Unmarshal(r.Bytes(), resp)
	if err != nil {
		return nil, err
	}
	if resp.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
		return nil, errors.New(resp.GetStatus().GetMessage())
	}
	return resp, nil
}
