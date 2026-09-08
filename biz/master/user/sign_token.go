package user

import (
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
)

func SignTokenHandler(ctx *app.Context, req *pb.SignTokenRequest) (*pb.SignTokenResponse, error) {
	var (
		userInfo    = common.GetUserInfo(ctx)
		permissions = req.GetPermissions()
		expiresIn   = req.GetExpiresIn()
		cfg         = ctx.GetApp().GetConfig()
	)

	token, err := utils.GetJwtTokenFromMap(conf.JWTSecret(cfg),
		time.Now().Unix(),
		int64(expiresIn),
		map[string]interface{}{
			defs.UserIDKey:                   userInfo.GetUserID(),
			defs.TokenPayloadKey_Permissions: permissions,
		})
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("get jwt token failed, req: [%s]", req.String())
		return nil, err
	}

	logger.Logger(ctx).Infof("get jwt token success, req: [%s]", req.String())

	return &pb.SignTokenResponse{
		Token: lo.ToPtr(token),
		Status: &pb.Status{
			Code:    pb.RespCode_RESP_CODE_SUCCESS,
			Message: "ok",
		},
	}, nil
}
