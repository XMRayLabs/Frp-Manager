package rpc

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"google.golang.org/protobuf/proto"
)

func nodeRequest(cfg conf.Config, token, path string, input, output proto.Message) error {
	body, err := proto.Marshal(input)
	if err != nil {
		return err
	}
	response, err := httpCli(cfg).R().SetHeader("Content-Type", "application/x-protobuf").SetHeader(defs.AuthorizationKey, token).SetBodyBytes(body).Post(conf.GetAPIURL(cfg) + path)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("panel returned HTTP %d", response.StatusCode)
	}
	return proto.Unmarshal(response.Bytes(), output)
}

func JoinServer(cfg conf.Config, id, token string) (*pb.Client, error) {
	get := func(id string) (*pb.Client, error) {
		response := &pb.GetServerResponse{}
		if err := nodeRequest(cfg, token, "/api/v1/server/get", &pb.GetServerRequest{ServerId: &id}, response); err != nil {
			return nil, err
		}
		if response.GetStatus() == nil || response.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS || response.GetServer().GetSecret() == "" {
			return nil, fmt.Errorf("get server: %s", response.GetStatus().GetMessage())
		}
		server := response.GetServer()
		return &pb.Client{Id: server.Id, Secret: server.Secret}, nil
	}
	if node, err := get(id); err == nil {
		return node, nil
	}
	response := &pb.InitServerResponse{}
	// The panel determines the connecting address; it remains editable there.
	if err := nodeRequest(cfg, token, "/api/v1/server/init", &pb.InitServerRequest{ServerId: &id}, response); err != nil {
		return nil, err
	}
	if response.GetStatus() == nil || response.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
		return nil, fmt.Errorf("register server: %s", response.GetStatus().GetMessage())
	}
	return get(response.GetServerId())
}
