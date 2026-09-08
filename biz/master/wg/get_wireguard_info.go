package wg

import (
	"errors"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"google.golang.org/protobuf/proto"
)

func GetWireGuardRuntimeInfo(ctx *app.Context, req *pb.GetWireGuardRuntimeInfoRequest) (*pb.GetWireGuardRuntimeInfoResponse, error) {
	log := ctx.Logger().WithField("op", "GetWireGuardRuntimeInfo")

	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		log.Errorf("invalid user")
		return nil, errors.New("invalid user")
	}

	wgRecord, err := dao.NewQuery(ctx).GetWireGuardByID(userInfo, uint(req.GetId()))
	if err != nil {
		log.WithError(err).Errorf("get wireguard by id failed, clientId: [%s], id: [%d]", req.GetClientId(), req.GetId())
		return nil, errors.New("get wireguard by id failed")
	}

	runtimeInfo, ok := ctx.GetApp().GetNetworkTopologyCache().GetRuntimeInfo(uint(wgRecord.ID))
	if !ok || runtimeInfo == nil {
		return &pb.GetWireGuardRuntimeInfoResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "runtime info has not been reported yet"},
		}, nil
	}
	runtimeInfo = proto.Clone(runtimeInfo).(*pb.WGDeviceRuntimeInfo)
	runtimeInfo.ClientId = wgRecord.ClientID
	runtimeInfo.VirtualIp = wgRecord.LocalAddress

	log.Debugf("get wireguard runtime info success with clientId: [%s], interfaceName: [%s], runtimeInfo: [%s]",
		wgRecord.ClientID, wgRecord.Name, runtimeInfo.String())

	return &pb.GetWireGuardRuntimeInfoResponse{
		Status:              &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		WgDeviceRuntimeInfo: runtimeInfo,
	}, nil
}

func GetWireGuardRuntimeInfos(ctx *app.Context, req *pb.GetWireGuardRuntimeInfosRequest) (*pb.GetWireGuardRuntimeInfosResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() || req.GetNetworkId() == 0 {
		return &pb.GetWireGuardRuntimeInfosResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid request"},
		}, nil
	}

	records, err := dao.NewQuery(ctx).GetWireGuardsByNetworkID(userInfo, uint(req.GetNetworkId()))
	if err != nil {
		return nil, err
	}

	runtimeInfos := make(map[uint32]*pb.WGDeviceRuntimeInfo, len(records))
	cache := ctx.GetApp().GetNetworkTopologyCache()
	for _, record := range records {
		if record == nil {
			continue
		}
		runtimeInfo, ok := cache.GetRuntimeInfo(uint(record.ID))
		if !ok || runtimeInfo == nil {
			continue
		}
		cloned := proto.Clone(runtimeInfo).(*pb.WGDeviceRuntimeInfo)
		cloned.ClientId = record.ClientID
		cloned.VirtualIp = record.LocalAddress
		runtimeInfos[uint32(record.ID)] = cloned
	}

	return &pb.GetWireGuardRuntimeInfosResponse{
		Status:       &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		RuntimeInfos: runtimeInfos,
	}, nil
}
