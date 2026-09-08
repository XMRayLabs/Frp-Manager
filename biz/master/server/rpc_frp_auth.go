package server

import (
	"context"
	"fmt"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/cache"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func FRPAuth(ctx *app.Context, req *pb.FRPAuthRequest) (*pb.FRPAuthResponse, error) {
	logger.Logger(ctx).Infof("frpc auth, req: [%+v]", req)
	var (
		err error
	)

	userToken, err := cache.Get().Get([]byte(req.User))
	if err != nil {
		u, err := dao.NewQuery(ctx).GetUserByUserName(req.User)
		if err != nil || u == nil {
			logger.Logger(context.Background()).WithError(err).Errorf("invalid user: %s", req.User)
			return &pb.FRPAuthResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
				Ok:     false,
			}, fmt.Errorf("invalid user: %s", req.User)
		}
		cache.Get().Set([]byte(u.GetUserName()), []byte(u.GetToken()), 0)
		userToken = []byte(u.GetToken())
	}

	if string(userToken) != req.GetToken() {
		logger.Logger(ctx).Error("invalid token")
		return &pb.FRPAuthResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid token"},
			Ok:     false,
		}, fmt.Errorf("invalid token")
	}

	return &pb.FRPAuthResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		Ok:     true,
	}, nil
}
