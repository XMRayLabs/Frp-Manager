package user

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cast"

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

	if expiresIn <= 0 || expiresIn > 365*24*60*60 {
		return nil, fmt.Errorf("token lifetime must be between 1 second and 1 year")
	}
	parents, err := common.GetTokenPermission(ctx)
	if err != nil {
		return nil, err
	}
	// Regex inclusion cannot be safely inferred. Restricted callers may delegate
	// only exact existing paths (or a subset of an explicit wildcard).
	unrestricted := false
	for _, parent := range parents {
		if parent.Method == "*" && parent.Path == "*" {
			unrestricted = true
		}
	}
	for _, child := range permissions {
		if child == nil || child.GetMethod() == "" || child.GetPath() == "" {
			return nil, fmt.Errorf("invalid token permission")
		}
		covered := false
		for _, parent := range parents {
			if (parent.Method == "*" || parent.Method == child.GetMethod()) && (parent.Path == "*" || parent.Path == child.GetPath()) {
				covered = true
				break
			}
		}
		if !covered {
			return nil, fmt.Errorf("requested permissions exceed the current token scope")
		}
	}
	claims, err := utils.ParseToken(conf.JWTSecret(cfg), common.GetTokenString(ctx))
	if err != nil {
		return nil, err
	}
	if !unrestricted || cast.ToBool(claims["api_token"]) {
		exp, err := jwt.MapClaims(claims).GetExpirationTime()
		if err != nil || exp == nil || time.Now().Add(time.Duration(expiresIn)*time.Second).After(exp.Time) {
			return nil, fmt.Errorf("delegated token must not outlive its parent")
		}
	}

	token, err := utils.GetJwtTokenFromMap(conf.JWTSecret(cfg),
		time.Now().Unix(),
		int64(expiresIn),
		map[string]interface{}{
			defs.UserIDKey:                   userInfo.GetUserID(),
			"api_token":                      true,
			"session_version":                userInfo.GetSessionVersion(),
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
