package wg

import (
	"errors"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

func DeleteNetwork(ctx *app.Context, req *pb.DeleteNetworkRequest) (*pb.DeleteNetworkResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	id := uint(req.GetId())
	if id == 0 {
		return nil, errors.New("invalid id")
	}
	if err := dao.NewMutation(ctx).DeleteNetwork(userInfo, id); err != nil {
		return nil, err
	}
	return &pb.DeleteNetworkResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
	}, nil
}
