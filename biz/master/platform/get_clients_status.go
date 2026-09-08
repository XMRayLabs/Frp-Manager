package platform

import (
	"context"
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
)

const (
	statusProbeConcurrency = 32
	statusProbeInterval    = 30 * time.Second
	statusProbeTimeout     = 3 * time.Second
)

var statusProbeSlots = make(chan struct{}, statusProbeConcurrency)

func GetClientsStatus(c *app.Context, req *pb.GetClientsStatusRequest) (*pb.GetClientsStatusResponse, error) {
	userInfo := common.GetUserInfo(c)
	if !userInfo.Valid() || req == nil || len(req.GetClientIds()) == 0 || req.GetClientType() == pb.ClientType_CLIENT_TYPE_UNSPECIFIED {
		return &pb.GetClientsStatusResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "request invalid"},
		}, nil
	}

	allowedIDs, err := getAllowedStatusIDs(c, userInfo, req)
	if err != nil {
		return nil, err
	}

	mgr := c.GetApp().GetClientsManager()
	resps := make(map[string]*pb.ClientStatus, len(allowedIDs))
	expectedType := connectorType(req.GetClientType())
	for _, clientID := range req.GetClientIds() {
		if _, allowed := allowedIDs[clientID]; !allowed {
			continue
		}

		connector := mgr.Get(clientID)
		if connector == nil || connector.CliType != expectedType {
			resps[clientID] = offlineStatus(req.GetClientType(), clientID)
			continue
		}

		status := &pb.ClientStatus{
			ClientType: req.GetClientType(),
			ClientId:   clientID,
			Status:     pb.ClientStatus_STATUS_ONLINE,
			Ping:       -1,
			Addr:       lo.ToPtr(mgr.ClientAddr(clientID)),
		}
		if connectTime, ok := mgr.ConnectTime(clientID); ok {
			status.ConnectTime = lo.ToPtr(connectTime.UnixMilli())
		}
		if snapshot, ok := mgr.GetRuntimeSnapshot(clientID); ok {
			status.Ping = snapshot.Ping
			status.Version = snapshot.Version
			if !snapshot.Healthy {
				status.Status = pb.ClientStatus_STATUS_ERROR
			}
		}
		resps[clientID] = status
		scheduleStatusProbe(c.GetApp(), clientID, connector)
	}

	return &pb.GetClientsStatusResponse{
		Status:  &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		Clients: resps,
	}, nil
}

func getAllowedStatusIDs(
	c *app.Context,
	userInfo models.UserInfo,
	req *pb.GetClientsStatusRequest,
) (map[string]struct{}, error) {
	allowed := make(map[string]struct{}, len(req.GetClientIds()))
	q := dao.NewQuery(c)

	switch req.GetClientType() {
	case pb.ClientType_CLIENT_TYPE_FRPC:
		clients, err := q.GetClientsByClientIDs(userInfo, req.GetClientIds())
		if err != nil {
			return nil, err
		}
		for _, client := range clients {
			allowed[client.ClientID] = struct{}{}
		}
	case pb.ClientType_CLIENT_TYPE_FRPS:
		servers, err := q.GetServersByServerIDs(userInfo, req.GetClientIds())
		if err != nil {
			return nil, err
		}
		for _, server := range servers {
			allowed[server.ServerID] = struct{}{}
		}
	default:
		return allowed, nil
	}
	return allowed, nil
}

func connectorType(clientType pb.ClientType) string {
	if clientType == pb.ClientType_CLIENT_TYPE_FRPS {
		return defs.CliTypeServer
	}
	return defs.CliTypeClient
}

func offlineStatus(clientType pb.ClientType, clientID string) *pb.ClientStatus {
	return &pb.ClientStatus{
		ClientType: clientType,
		ClientId:   clientID,
		Status:     pb.ClientStatus_STATUS_OFFLINE,
		Ping:       -1,
	}
}

func scheduleStatusProbe(appInstance app.Application, clientID string, connector *defs.Connector) {
	mgr := appInstance.GetClientsManager()
	// Reserve capacity before spawning: a large fleet must not create an unbounded
	// queue of goroutines waiting for a probe slot. Later polls retry skipped nodes.
	select {
	case statusProbeSlots <- struct{}{}:
	default:
		return
	}
	if !mgr.TryStartStatusProbe(clientID, statusProbeInterval) {
		<-statusProbeSlots
		return
	}

	go func() {
		defer func() { <-statusProbeSlots }()

		if mgr.Get(clientID) != connector {
			mgr.FinishStatusProbe(clientID, connector, -1, nil, false)
			return
		}

		probeCtx, cancel := context.WithTimeout(context.Background(), statusProbeTimeout)
		defer cancel()
		startedAt := time.Now()
		resp, err := rpc.CallClient(app.NewContext(probeCtx, appInstance), clientID, pb.Event_EVENT_PING, &pb.CommonRequest{})
		ping := int32(time.Since(startedAt).Milliseconds())
		if err != nil || resp == nil {
			mgr.FinishStatusProbe(clientID, connector, ping, nil, false)
			logger.Logger(context.Background()).WithError(err).Debugf("client status probe failed, client id: [%s]", clientID)
			return
		}

		version := &pb.ClientVersion{}
		if err := proto.Unmarshal(resp.GetData(), version); err != nil {
			mgr.FinishStatusProbe(clientID, connector, ping, nil, false)
			return
		}
		mgr.FinishStatusProbe(clientID, connector, ping, version, true)
	}()
}
