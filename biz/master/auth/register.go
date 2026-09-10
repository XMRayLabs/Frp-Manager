package auth

import (
	"fmt"
	"strings"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type registerJSONRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	InviteCode string `json:"invite_code"`
}

func RegisterGinHandler(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := registerJSONRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ErrResp(c, &pb.RegisterResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
			}, err.Error())
			return
		}

		resp, err := registerUser(app.NewContext(c, appInstance), req)
		if err != nil {
			common.ErrResp(c, resp, err.Error())
			return
		}
		common.OKResp(c, resp)
	}
}

func RegisterHandler(c *app.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return registerUser(c, registerJSONRequest{
		Username: req.GetUsername(),
		Password: req.GetPassword(),
		Email:    req.GetEmail(),
	})
}

func registerUser(c *app.Context, req registerJSONRequest) (*pb.RegisterResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	for _, err := range []error{
		utils.ValidateUserName(req.Username),
		utils.ValidateEmail(req.Email),
		utils.ValidatePassword(req.Password, req.Username),
	} {
		if err != nil {
			return &pb.RegisterResponse{
				Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
			}, err
		}
	}

	userCount, err := dao.NewQuery(c).AdminCountUsers()
	if err != nil {
		return &pb.RegisterResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
		}, err
	}

	if !registerEnabled(c) {
		return &pb.RegisterResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "register is disabled"},
		}, fmt.Errorf("register is disabled")
	}

	tenantID := defs.DefaultAdminUserID

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return &pb.RegisterResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
		}, err
	}

	newUser := &models.UserEntity{
		UserName: req.Username,
		Password: hashedPassword,
		Email:    req.Email,
		Status:   models.STATUS_NORMAL,
		Role:     defs.UserRole_Normal,
		TenantID: tenantID,
		Token:    uuid.New().String(),
	}

	if userCount == 0 {
		newUser.Role = defs.UserRole_Admin
		newUser.TenantID = defs.DefaultAdminUserID
	}

	err = c.GetApp().GetDBManager().GetDefaultDB().Transaction(func(tx *gorm.DB) error {
		if userCount > 0 {
			if inviteRequired(c) || strings.TrimSpace(req.InviteCode) != "" {
				invite, err := consumeGroupInvite(tx, req.InviteCode)
				if err != nil {
					return err
				}
				newUser.TenantID, newUser.LanguageGroupID = invite.TenantID, invite.LanguageGroupID
			} else {
				id, err := models.DefaultLanguageGroup(tx, newUser.TenantID)
				if err != nil {
					return err
				}
				newUser.LanguageGroupID = id
			}
		}
		if userCount == 0 {
			if _, err := models.DefaultLanguageGroup(tx, newUser.TenantID); err != nil {
				return err
			}
			if err := tx.Create(&models.SystemSetting{Key: "language_groups_v1", TenantID: newUser.TenantID, Value: uuid.NewString()}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.User{UserEntity: newUser}).Error
	})
	if err != nil {
		return &pb.RegisterResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()},
		}, err
	}

	return &pb.RegisterResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
