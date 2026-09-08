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

func UpdateWireGuard(ctx *app.Context, req *pb.UpdateWireGuardRequest) (*pb.UpdateWireGuardResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return nil, errors.New("invalid user")
	}
	cfg := req.GetWireguardConfig()
	if cfg == nil || cfg.GetId() == 0 || len(cfg.GetClientId()) == 0 || len(cfg.GetInterfaceName()) == 0 || len(cfg.GetPrivateKey()) == 0 || len(cfg.GetLocalAddress()) == 0 {
		return nil, errors.New("invalid wireguard config")
	}
	q := dao.NewQuery(ctx)
	m := dao.NewMutation(ctx)

	model := &models.WireGuard{}
	model.FromPB(cfg)
	model.UserId = uint32(userInfo.GetUserID())
	model.TenantId = uint32(userInfo.GetTenantID())

	if err := m.UpdateWireGuard(userInfo, uint(cfg.GetId()), model); err != nil {
		return nil, err
	}

	// 绔偣璧嬪€间笌瑙ｇ粦
	// 1) 璇诲彇褰撳墠缁戝畾鐨勭鐐?
	currentList, err := q.ListEndpointsWithFilters(userInfo, 1, 1000, "", uint(cfg.GetId()), "")
	if err != nil {
		return nil, err
	}
	currentSet := lo.SliceToMap(currentList, func(e *models.Endpoint) (uint, struct{}) { return uint(e.ID), struct{}{} })

	// 2) 澶勭悊鏈閰嶇疆涓殑绔偣锛氬瓨鍦ㄥ垯鏇存柊骞剁粦瀹氾紱涓嶅瓨鍦ㄥ垯鍒涘缓缁戝畾
	newSet := map[uint]struct{}{}
	for _, ep := range cfg.GetAdvertisedEndpoints() {
		if ep == nil {
			continue
		}
		if ep.GetId() > 0 {
			exist, err := q.GetEndpointByID(userInfo, uint(ep.GetId()))
			if err != nil {
				return nil, err
			}
			// 蹇呴』灞炰簬鐩稿悓 client
			if exist.ClientID != cfg.GetClientId() {
				return nil, errors.New("endpoint client mismatch")
			}
			exist.WireGuardID = uint(cfg.GetId())

			if err := m.UpdateEndpoint(userInfo, uint(exist.ID), exist.EndpointEntity); err != nil {
				return nil, err
			}
			newSet[uint(exist.ID)] = struct{}{}
		} else {

			entity := &models.Endpoint{}
			entity.FromPB(ep)
			entity.WireGuardID = uint(cfg.GetId())
			entity.ClientID = cfg.GetClientId()

			if err := m.CreateEndpoint(userInfo, entity.EndpointEntity); err != nil {
				return nil, err
			}
		}
	}

	// 3) 瑙ｇ粦鏈鏈寘鍚殑绔偣锛堝皢 WireGuardID 缃?0锛?
	for id := range currentSet {
		if _, ok := newSet[id]; ok {
			continue
		}
		exist, err := q.GetEndpointByID(userInfo, id)
		if err != nil {
			return nil, err
		}
		entity := &models.Endpoint{}
		entity.FromPB(exist.ToPB())
		entity.WireGuardID = 0
		if err := m.UpdateEndpoint(userInfo, id, entity.EndpointEntity); err != nil {
			return nil, err
		}
	}
	return &pb.UpdateWireGuardResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"}, WireguardConfig: cfg}, nil
}
