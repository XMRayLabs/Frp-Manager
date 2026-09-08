package wg

import (
	"errors"
	"fmt"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/wg"
)

func GetNetworkTopology(ctx *app.Context, req *pb.GetNetworkTopologyRequest) (*pb.GetNetworkTopologyResponse, error) {
	log := ctx.Logger().WithField("op", "GetNetworkTopology")

	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}

	networkID := uint(req.GetId())
	if networkID == 0 {
		return nil, errors.New("invalid id")
	}

	q := dao.NewQuery(ctx)

	peers, err := q.GetWireGuardsByNetworkID(userInfo, networkID)
	if err != nil {
		log.WithError(err).Errorf("failed to get wireguard peers by network id: %d", networkID)
		return nil, err
	}
	links, err := q.ListWireGuardLinksByNetwork(userInfo, networkID)
	if err != nil {
		log.WithError(err).Errorf("failed to get wireguard links by network id: %d", networkID)
		return nil, err
	}

	if len(peers) == 0 {
		log.Errorf("no wireguard peers found")
		return nil, fmt.Errorf("no wireguard peers found")
	}

	policy := wg.DefaultRoutingPolicy(
		wg.NewACL().LoadFromPB(peers[0].Network.ACL.Data),
		ctx.GetApp().GetNetworkTopologyCache(),
		ctx.GetApp().GetClientsManager(),
	)

	if req.GetSpf() {
		// SPF 妯″紡锛氬睍绀衡€滅湡瀹炰笅鍙戠殑璺敱琛ㄢ€濓紙鍗?PeerConfig.AllowedIps锛夛紝纭繚涓庡疄闄呬竴鑷淬€?
		peerCfgs, allEdges, err := wg.PlanAllowedIPs(peers, links, policy)
		if err != nil {
			log.WithError(err).Errorf("failed to plan allowed ips")
			return nil, err
		}
		adjs := peerConfigsToPBAdjs(peerCfgs, allEdges)

		return &pb.GetNetworkTopologyResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
			Adjs:   adjs,
		}, nil
	}

	resp, err := wg.NewDijkstraAllowedIPsPlanner(policy).BuildGraph(peers, links)
	if err != nil {
		log.WithError(err).Errorf("failed to build graph")
		return nil, err
	}
	adjs := adjsToPB(resp)

	return &pb.GetNetworkTopologyResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		Adjs:   adjs,
	}, nil
}
