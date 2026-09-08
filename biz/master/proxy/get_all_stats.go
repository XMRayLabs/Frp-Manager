package proxy

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

func GetAllProxyStats(ctx *app.Context, _ *pb.GetAllProxyStatsRequest) (*pb.GetAllProxyStatsResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return &pb.GetAllProxyStatsResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	stats, err := dao.NewQuery(ctx).GetAllProxyStats(userInfo)
	if err != nil {
		return nil, err
	}
	return &pb.GetAllProxyStatsResponse{
		Status:     &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		ProxyInfos: convertProxyStatsList(stats),
	}, nil
}
