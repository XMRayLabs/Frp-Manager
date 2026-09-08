package wg

import (
	"errors"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

func CreateEndpoint(ctx *app.Context, req *pb.CreateEndpointRequest) (*pb.CreateEndpointResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	e := req.GetEndpoint()
	if e == nil || len(e.GetHost()) == 0 || e.GetPort() == 0 {
		return nil, errors.New("invalid endpoint params")
	}

	entity := &models.EndpointEntity{Host: e.GetHost(), Port: e.GetPort(), ClientID: e.GetClientId(), Type: e.GetType(), Uri: e.GetUri()}
	if err := dao.NewMutation(ctx).CreateEndpoint(userInfo, entity); err != nil {
		return nil, err
	}
	return &pb.CreateEndpointResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"}, Endpoint: &pb.Endpoint{Id: 0, Host: entity.Host, Port: entity.Port, ClientId: entity.ClientID}}, nil
}
