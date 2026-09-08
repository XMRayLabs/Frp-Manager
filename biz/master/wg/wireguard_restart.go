package wg

import (
	"errors"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

// RestartWireGuard 閲嶅惎鎸囧畾 WireGuard 鎺ュ彛锛岄€氱煡瀵瑰簲瀹㈡埛绔墽琛岄噸鍚?
func RestartWireGuard(ctx *app.Context, req *pb.RestartWireGuardRequest) (*pb.RestartWireGuardResponse, error) {
	log := ctx.Logger().WithField("op", "RestartWireGuard")

	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}

	id := req.GetId()
	if id == 0 {
		return nil, errors.New("invalid id")
	}

	wgRecord, err := dao.NewQuery(ctx).GetWireGuardByID(userInfo, uint(id))
	if err != nil {
		log.WithError(err).Errorf("get wireguard by id failed")
		return nil, err
	}

	go func() {
		// 濂藉儚鐩存帴restart鏈夌偣闂
		// 涓嶅鐩存帴鍒犳帀閲嶅紑
		// resp := &pb.RestartWireGuardResponse{}
		// if err := rpc.CallClientWrapper(ctx, wgRecord.ClientID, pb.Event_EVENT_RESTART_WIREGUARD, &pb.RestartWireGuardRequest{
		// 	Id:            lo.ToPtr(uint32(wgRecord.ID)),
		// 	ClientId:      lo.ToPtr(wgRecord.ClientID),
		// 	InterfaceName: lo.ToPtr(wgRecord.Name),
		// }, resp); err != nil {
		log.WithError(err).Warnf("restart wireguard event send to client failed, fallback to delete and create")
		if err := emitDeleteWireGuardEvent(ctx, wgRecord); err != nil {
			log.WithError(err).Errorf("emit delete wireguard event failed")
			return
		}
		if err := emitCreateWireGuardEvent(ctx, wgRecord.ToPB(), wgRecord.Network.NetworkEntity); err != nil {
			log.WithError(err).Errorf("emit create wireguard event failed")
			return
		}
		log.Infof("emit delete and create wireguard event success, client id: [%s], wireguard interface: [%s]", wgRecord.ClientID, wgRecord.Name)
		// }
	}()

	log.Infof("restart wireguard event send to client success, client id: [%s], wireguard interface: [%s]", wgRecord.ClientID, wgRecord.Name)

	return &pb.RestartWireGuardResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"}}, nil
}
