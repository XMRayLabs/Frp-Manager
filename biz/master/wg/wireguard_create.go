package wg

import (
	"errors"
	"net/netip"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	wgsvc "github.com/Sakurame1/frp-manager/services/wg"
	"github.com/Sakurame1/frp-manager/utils"
)

// Create/Update/Get/List WireGuard 鍩轰簬 pb.WireGuardConfig
// 灏?pb 鏄犲皠鍒?models.WireGuard + models.Endpoint 鍒楄〃
func CreateWireGuard(ctx *app.Context, req *pb.CreateWireGuardRequest) (*pb.CreateWireGuardResponse, error) {
	log := ctx.Logger().WithField("op", "CreateWireGuard")

	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	cfg := req.GetWireguardConfig()
	if cfg == nil || len(cfg.GetClientId()) == 0 || len(cfg.GetInterfaceName()) == 0 || len(cfg.GetLocalAddress()) == 0 {
		return nil, errors.New("invalid wireguard config")
	}
	q := dao.NewQuery(ctx)
	m := dao.NewMutation(ctx)

	ips, err := q.GetWireGuardLocalAddressesByNetworkID(userInfo, uint(cfg.GetNetworkId()))
	if err != nil {
		log.WithError(err).Errorf("get wireguard local addresses by network id failed")
		return nil, err
	}

	network, err := q.GetNetworkByID(userInfo, uint(cfg.GetNetworkId()))
	if err != nil {
		log.WithError(err).Errorf("get network by id failed")
		return nil, err
	}

	newIpStr, err := utils.AllocateIP(network.CIDR, ips, cfg.GetLocalAddress())
	if err != nil {
		log.WithError(err).Errorf("allocate ip failed")
		return nil, err
	}

	newIp, err := netip.ParseAddr(newIpStr)
	if err != nil {
		log.WithError(err).Errorf("parse ip failed")
		return nil, err
	}

	networkCidr, err := netip.ParsePrefix(network.CIDR)
	if err != nil {
		log.WithError(err).Errorf("parse network cidr failed")
		return nil, err
	}

	newIpCidr := netip.PrefixFrom(newIp, networkCidr.Bits())

	keys := wgsvc.GenerateKeys()

	wgModel := &models.WireGuard{}
	wgModel.FromPB(cfg)
	wgModel.UserId = uint32(userInfo.GetUserID())
	wgModel.TenantId = uint32(userInfo.GetTenantID())
	wgModel.PrivateKey = keys.PrivateKeyBase64
	wgModel.LocalAddress = newIpCidr.String()

	log.Debugf("create wireguard with config: %+v", wgModel)

	if err := m.CreateWireGuard(userInfo, wgModel); err != nil {
		return nil, err
	}

	// 澶勭悊绔偣锛氫紭鍏堝鐢ㄥ凡瀛樺湪鐨?endpoint锛堥€氳繃 id锛夛紝鍚﹀垯鍒涘缓鏂扮鐐瑰苟缁戝畾鍒拌 WireGuard
	for _, ep := range cfg.GetAdvertisedEndpoints() {
		if ep == nil {
			continue
		}
		if ep.GetId() > 0 {
			// 澶嶇敤鐜版湁 endpoint锛岃姹傚綊灞炲悓涓€ client
			exist, err := q.GetEndpointByID(userInfo, uint(ep.GetId()))
			if err != nil {
				return nil, err
			}
			if exist.ClientID != cfg.GetClientId() {
				return nil, errors.New("endpoint client mismatch")
			}
			exist.WireGuardID = wgModel.ID

			if err := m.UpdateEndpoint(userInfo, uint(exist.ID), exist.EndpointEntity); err != nil {
				return nil, err
			}
		} else {
			// 鍒涘缓骞剁粦瀹氭柊绔偣
			newEp := &models.Endpoint{}
			newEp.FromPB(ep)
			newEp.ClientID = cfg.GetClientId()
			newEp.WireGuardID = wgModel.ID
			if err := m.CreateEndpoint(userInfo, newEp.EndpointEntity); err != nil {
				return nil, err
			}
		}
	}

	go func() {
		if err := emitCreateWireGuardEvent(ctx, cfg, network.NetworkEntity); err != nil {
			log.WithError(err).Errorf("emit create wireguard event failed")
		}
		log.Infof("emit create wireguard event success, client id: [%s], wireguard interface: [%s]", cfg.GetClientId(), cfg.GetInterfaceName())
	}()

	return &pb.CreateWireGuardResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"}, WireguardConfig: cfg}, nil
}

func emitCreateWireGuardEvent(ctx *app.Context, cfg *pb.WireGuardConfig, network *models.NetworkEntity) error {
	log := ctx.Logger().WithField("op", "emitCreateWireGuardEvent")

	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return errors.New("invalid user")
	}

	q := dao.NewQuery(ctx)

	peers, err := q.GetWireGuardsByNetworkID(userInfo, uint(cfg.GetNetworkId()))
	if err != nil {
		log.WithError(err).Errorf("get wireguards by network id failed")
		return err
	}
	links, err := q.ListWireGuardLinksByNetwork(userInfo, uint(cfg.GetNetworkId()))
	if err != nil {
		log.WithError(err).Errorf("get wireguard links by network id failed")
		return err
	}

	peerConfigs, adjs, err := wgsvc.PlanAllowedIPs(
		peers,
		links,
		wgsvc.DefaultRoutingPolicy(
			wgsvc.NewACL().LoadFromPB(network.ACL.Data),
			ctx.GetApp().GetNetworkTopologyCache(),
			ctx.GetApp().GetClientsManager(),
		))
	if err != nil {
		log.WithError(err).Errorf("build peer configs for network failed")
		return err
	}

	for _, peer := range peers {
		if peer.ClientID == cfg.GetClientId() {
			if err := emitCreateWireGuardEventToClient(ctx, peer, peerConfigs[peer.ID], adjs); err != nil {
				log.WithError(err).Errorf("update config to client failed")
			}
			continue
		}

		if err := emitPatchWireGuardEventToClient(ctx, peer, peerConfigs[peer.ID], adjs); err != nil {
			log.WithError(err).Errorf("add wireguard event send to client error")
			continue
		}

		log.Debugf("update config to client success, client id: [%s], wireguard interface: [%s]", peer.ClientID, peer.Name)
	}

	return nil
}

func emitCreateWireGuardEventToClient(ctx *app.Context, peer *models.WireGuard, peerConfigs []*pb.WireGuardPeerConfig, adjs map[uint][]wgsvc.Edge) error {
	log := ctx.Logger().WithField("op", "updateConfigToClient")
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return errors.New("invalid user")
	}

	cfg := peer.ToPB()
	cfg.Peers = peerConfigs
	cfg.Adjs = adjsToPB(adjs)
	resp := &pb.CreateWireGuardResponse{}

	req := &pb.CreateWireGuardRequest{
		WireguardConfig: cfg,
	}

	err := rpc.CallClientWrapper(ctx, peer.ClientID, pb.Event_EVENT_CREATE_WIREGUARD, req, resp)
	if err != nil {
		log.WithError(err).Errorf("create wireguard event send to client error")
		return err
	}

	log.Infof("create wireguard event send to client success, client id: [%s], wireguard interface: [%s]",
		peer.ClientID, peer.Name)
	return nil
}

func emitPatchWireGuardEventToClient(ctx *app.Context, peer *models.WireGuard, peerConfigs []*pb.WireGuardPeerConfig, adjs map[uint][]wgsvc.Edge) error {
	log := ctx.Logger().WithField("op", "patchWireGuardToClient")
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return errors.New("invalid user")
	}

	cfg := peer.ToPB()
	cfg.Peers = peerConfigs
	cfg.Adjs = adjsToPB(adjs)

	resp := &pb.UpdateWireGuardResponse{}
	req := &pb.UpdateWireGuardRequest{
		WireguardConfig: cfg,
		UpdateType:      pb.UpdateWireGuardRequest_UPDATE_TYPE_PATCH_PEERS.Enum(),
	}

	err := rpc.CallClientWrapper(ctx, peer.ClientID, pb.Event_EVENT_UPDATE_WIREGUARD, req, resp)
	if err != nil {
		log.WithError(err).Errorf("add wireguard event send to client error")
		return err
	}

	log.Infof("add wireguard event send to client success, client id: [%s], wireguard interface: [%s]",
		peer.ClientID, peer.Name)
	return nil
}
