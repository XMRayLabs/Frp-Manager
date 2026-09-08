package server

import (
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

func PushProxyInfo(ctx *app.Context, req *pb.PushProxyInfoReq) (*pb.PushProxyInfoResp, error) {
	var srv *models.ServerEntity
	var err error

	if srv, err = ValidateServerRequest(ctx, req.GetBase()); err != nil {
		return nil, err
	}

	if err = dao.NewMutation(ctx).AdminUpdateProxyStats(srv, req.GetProxyInfos()); err != nil {
		return nil, err
	}
	return &pb.PushProxyInfoResp{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
