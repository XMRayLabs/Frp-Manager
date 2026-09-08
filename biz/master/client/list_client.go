package client

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
)

func ListClientsHandler(ctx *app.Context, req *pb.ListClientsRequest) (*pb.ListClientsResponse, error) {
	logger.Logger(ctx).Infof("list client, req: [%+v]", req)

	var (
		userInfo = common.GetUserInfo(ctx)
	)

	if !userInfo.Valid() {
		return &pb.ListClientsResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	var (
		page         = int(req.GetPage())
		pageSize     = int(req.GetPageSize())
		keyword      = req.GetKeyword()
		clients      []*models.ClientEntity
		err          error
		clientCounts int64
		hasKeyword   = len(keyword) > 0
	)

	if hasKeyword {
		clients, err = dao.NewQuery(ctx).ListClientsWithKeyword(userInfo, page, pageSize, keyword)
	} else {
		clients, err = dao.NewQuery(ctx).ListClients(userInfo, page, pageSize)
	}

	if err != nil {
		return nil, err
	}

	if hasKeyword {
		clientCounts, err = dao.NewQuery(ctx).CountClientsWithKeyword(userInfo, keyword)
	} else {
		clientCounts, err = dao.NewQuery(ctx).CountClients(userInfo)
	}

	if err != nil {
		return nil, err
	}

	clientIDs := lo.Map(clients, func(c *models.ClientEntity, _ int) string {
		return c.ClientID
	})
	shadowClientIDs, err := dao.NewQuery(ctx).GetClientIDsInShadowByClientIDs(userInfo, clientIDs)
	if err != nil {
		return nil, err
	}

	respClients := lo.Map(clients, func(c *models.ClientEntity, _ int) *pb.Client {
		respCli := &pb.Client{
			Id:        lo.ToPtr(c.ClientID),
			Secret:    lo.ToPtr(c.ConnectSecret),
			Config:    lo.ToPtr(string(c.ConfigContent)),
			ServerId:  lo.ToPtr(c.ServerID),
			Stopped:   lo.ToPtr(c.Stopped),
			Comment:   lo.ToPtr(c.Comment),
			ClientIds: shadowClientIDs[c.ClientID],
			Ephemeral: lo.ToPtr(c.Ephemeral),
		}
		if c.LastSeenAt != nil {
			respCli.LastSeenAt = lo.ToPtr(c.LastSeenAt.UnixMilli())
		}
		return respCli
	})

	return &pb.ListClientsResponse{
		Status:  &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		Clients: respClients,
		Total:   lo.ToPtr(int32(clientCounts)),
	}, nil
}
