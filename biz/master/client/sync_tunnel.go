package client

import (
	"context"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
)

func SyncTunnel(ctx *app.Context, userInfo models.UserInfo) error {
	clis, err := dao.NewQuery(ctx).GetAllClients(userInfo)
	if err != nil {
		return err
	}
	lo.ForEach(clis, func(cli *models.ClientEntity, _ int) {
		// GetAllClients includes resources visible through admin/group sharing.
		// Updating one account must only rotate that account's own credentials.
		if cli.UserID != userInfo.GetUserID() || cli.TenantID != userInfo.GetTenantID() || len(cli.ConfigContent) == 0 {
			return
		}
		cfg, err := cli.GetConfigContent()
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot get client config content, id: [%s]", cli.ClientID)
			return
		}

		cfg.User = userInfo.GetUserName()
		if cfg.Metadatas == nil {
			cfg.Metadatas = map[string]string{}
		}
		cfg.Metadatas[defs.FRPAuthTokenKey] = userInfo.GetToken()
		cfg.Metadatas[defs.FRPClientIDKey] = cli.ClientID
		if err := cli.SetConfigContent(*cfg); err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot set client config content, id: [%s]", cli.ClientID)
			return
		}

		if err := dao.NewMutation(ctx).UpdateClient(userInfo, cli); err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot update client, id: [%s]", cli.ClientID)
			return
		}
		logger.Logger(ctx).Infof("update client success, id: [%s]", cli.ClientID)
	})
	return nil
}
