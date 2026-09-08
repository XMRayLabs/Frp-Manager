package wg

import (
	"errors"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/samber/lo"
)

func GetEndpoint(ctx *app.Context, req *pb.GetEndpointRequest) (*pb.GetEndpointResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	id := uint(req.GetId())
	if id == 0 {
		return nil, errors.New("invalid id")
	}
	edp, err := dao.NewQuery(ctx).GetEndpointByID(userInfo, id)
	if err != nil {
		return nil, err
	}
	return &pb.GetEndpointResponse{
		Status:   &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		Endpoint: edp.ToPB(),
	}, nil
}

func ListEndpoints(ctx *app.Context, req *pb.ListEndpointsRequest) (*pb.ListEndpointsResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	page, pageSize := int(req.GetPage()), int(req.GetPageSize())
	keyword := req.GetKeyword()
	clientID := req.GetClientId()
	wireguardID := uint(req.GetWireguardId())
	list, err := dao.NewQuery(ctx).ListEndpointsWithFilters(userInfo, page, pageSize, clientID, wireguardID, keyword)
	if err != nil {
		return nil, err
	}
	count, err := dao.NewQuery(ctx).CountEndpointsWithFilters(userInfo, clientID, wireguardID, keyword)
	if err != nil {
		return nil, err
	}
	resp := lo.Map(list, func(item *models.Endpoint, _ int) *pb.Endpoint {
		return item.ToPB()
	})
	return &pb.ListEndpointsResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"}, Endpoints: resp, Total: lo.ToPtr(int32(count))}, nil
}
