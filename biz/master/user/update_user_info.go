package user

import (
	"context"

	"github.com/Sakurame1/frp-manager/biz/master/client"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func UpdateUserInfoHander(c *app.Context, req *pb.UpdateUserInfoRequest) (*pb.UpdateUserInfoResponse, error) {
	var (
		userInfo = common.GetUserInfo(c)
	)

	if !userInfo.Valid() {
		return &pb.UpdateUserInfoResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}
	newUserEntity := userInfo.(*models.UserEntity)
	newUserInfo := req.GetUserInfo()

	if newUserInfo.GetEmail() != "" {
		if err := utils.ValidateEmail(newUserInfo.GetEmail()); err != nil {
			return &pb.UpdateUserInfoResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
			}, err
		}
		newUserEntity.Email = newUserInfo.GetEmail()
	}

	if newUserInfo.GetRawPassword() != "" {
		if err := utils.ValidatePassword(newUserInfo.GetRawPassword(), newUserEntity.UserName); err != nil {
			return &pb.UpdateUserInfoResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
			}, err
		}
		hashedPassword, err := utils.HashPassword(newUserInfo.GetRawPassword())
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot hash password")
			return nil, err
		}
		newUserEntity.Password = hashedPassword
		newUserEntity.SessionVersion++
	}

	if newUserInfo.GetUserName() != "" {
		if err := utils.ValidateUserName(newUserInfo.GetUserName()); err != nil {
			return &pb.UpdateUserInfoResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
			}, err
		}
		newUserEntity.UserName = newUserInfo.GetUserName()
	}

	if newUserInfo.GetToken() != "" {
		newUserEntity.Token = newUserInfo.GetToken()
	}

	if err := dao.NewMutation(c).UpdateUser(userInfo, newUserEntity); err != nil {
		return &pb.UpdateUserInfoResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
		}, err
	}

	go func() {
		newUser, err := dao.NewQuery(app.NewContext(context.Background(), c.GetApp())).GetUserByUserID(userInfo.GetUserID())
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot get user")
			return
		}

		if err := client.SyncTunnel(c, newUser); err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot sync tunnel, user need to retry update")
		}
	}()

	return &pb.UpdateUserInfoResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
