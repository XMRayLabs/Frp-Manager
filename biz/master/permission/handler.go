package permission

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	authsvc "github.com/Sakurame1/frp-manager/biz/master/auth"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	rbacsvc "github.com/Sakurame1/frp-manager/services/rbac"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type permissionRequest struct {
	ObjType    string   `json:"obj_type"`
	ObjID      string   `json:"obj_id"`
	TargetType string   `json:"target_type"`
	TargetID   string   `json:"target_id"`
	Permission string   `json:"permission"`
	Actions    []string `json:"actions"`
}

type groupRequest struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Comment   string `json:"comment"`
}

type groupMemberRequest struct {
	GroupID string `json:"group_id"`
	UserID  int    `json:"user_id"`
}

type userAdminRequest struct {
	UserID   int    `json:"user_id"`
	UserName string `json:"user_name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   *int   `json:"status"`
}

type inviteCreateRequest struct {
	LanguageGroupID string `json:"language_group_id"`
	Code            string `json:"code"`
	MaxUses         int    `json:"max_uses"`
	ExpiresAt       *int64 `json:"expires_at"`
	Comment         string `json:"comment"`
}

type inviteUpdateRequest struct {
	ID       uint  `json:"id"`
	Disabled *bool `json:"disabled"`
}

type registerSettingRequest struct {
	RegisterEnabled *bool `json:"register_enabled"`
	InviteRequired  *bool `json:"invite_required"`
}

type resourcePermissionRequest struct {
	ObjType string `json:"obj_type"`
	ObjID   string `json:"obj_id"`
}

type resourcePermissionEntry struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Permission string `json:"permission"`
}

func Share(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := app.NewContext(c, appInstance)
		userInfo := common.GetUserInfo(c)
		if appInstance.GetPermManager() == nil {
			errJSON(c, http.StatusInternalServerError, fmt.Errorf("permission manager is not initialized"))
			return
		}
		req := permissionRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}

		objType, permissions, err := parsePermissionRequest(req)
		if err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if err := canShare(ctx, userInfo, objType, req.ObjID); err != nil {
			errJSON(c, http.StatusForbidden, err)
			return
		}

		if objType == defs.RBACObjServer {
			errJSON(c, 400, fmt.Errorf("请在语系管理中分配服务端"))
			return
		}
		if objType == defs.RBACObjClient {
			var node models.Client
			if err := appInstance.GetDBManager().GetDefaultDB().Where("client_id = ? AND tenant_id = ?", req.ObjID, userInfo.GetTenantID()).First(&node).Error; err != nil {
				errJSON(c, 400, err)
				return
			}
			var owner models.User
			if err := appInstance.GetDBManager().GetDefaultDB().Where("user_id = ?", node.UserID).First(&owner).Error; err != nil {
				errJSON(c, 400, err)
				return
			}
			if owner.LanguageGroupID == "" {
				errJSON(c, 400, fmt.Errorf("未归属语系的设备不参与共享"))
				return
			}
			var count int64
			if req.TargetType == string(defs.RBACSubjectUser) {
				appInstance.GetDBManager().GetDefaultDB().Model(&models.User{}).Where("user_id = ? AND tenant_id = ? AND language_group_id = ?", req.TargetID, userInfo.GetTenantID(), owner.LanguageGroupID).Count(&count)
			} else {
				appInstance.GetDBManager().GetDefaultDB().Model(&models.UserGroup{}).Where("group_id = ? AND tenant_id = ? AND language_group_id = ?", req.TargetID, userInfo.GetTenantID(), owner.LanguageGroupID).Count(&count)
			}
			if count != 1 {
				errJSON(c, 403, fmt.Errorf("只可向设备所属语系授权"))
				return
			}
		}
		for _, permission := range permissions {
			switch req.TargetType {
			case string(defs.RBACSubjectUser):
				var userID int
				if _, err := fmt.Sscan(req.TargetID, &userID); err != nil || userID <= 0 {
					errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid target user id"))
					return
				}
				if err := ensureTenantUser(appInstance, userInfo.GetTenantID(), userID); err != nil {
					errJSON(c, http.StatusBadRequest, err)
					return
				}
				for _, action := range expandPermission(permission) {
					if _, err := appInstance.GetPermManager().GrantUserPermission(userID, objType, req.ObjID, action, userInfo.GetTenantID()); err != nil {
						errJSON(c, http.StatusInternalServerError, err)
						return
					}
				}
			case string(defs.RBACSubjectGroup):
				if err := ensureTenantGroup(appInstance, userInfo.GetTenantID(), req.TargetID); err != nil {
					errJSON(c, http.StatusBadRequest, err)
					return
				}
				for _, action := range expandPermission(permission) {
					if _, err := appInstance.GetPermManager().GrantGroupPermission(req.TargetID, objType, req.ObjID, action, userInfo.GetTenantID()); err != nil {
						errJSON(c, http.StatusInternalServerError, err)
						return
					}
				}
			default:
				errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid target type"))
				return
			}
		}
		okJSON(c, gin.H{"shared": true})
	}
}

func Revoke(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := app.NewContext(c, appInstance)
		userInfo := common.GetUserInfo(c)
		if appInstance.GetPermManager() == nil {
			errJSON(c, http.StatusInternalServerError, fmt.Errorf("permission manager is not initialized"))
			return
		}
		req := permissionRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}

		objType, permissions, err := parsePermissionRequest(req)
		if err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if err := canShare(ctx, userInfo, objType, req.ObjID); err != nil {
			errJSON(c, http.StatusForbidden, err)
			return
		}

		for _, permission := range permissions {
			switch req.TargetType {
			case string(defs.RBACSubjectUser):
				var userID int
				if _, err := fmt.Sscan(req.TargetID, &userID); err != nil || userID <= 0 {
					errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid target user id"))
					return
				}
				for _, action := range expandPermission(permission) {
					if _, err := appInstance.GetPermManager().RevokeUserPermission(userID, objType, req.ObjID, action, userInfo.GetTenantID()); err != nil {
						errJSON(c, http.StatusInternalServerError, err)
						return
					}
				}
			case string(defs.RBACSubjectGroup):
				for _, action := range expandPermission(permission) {
					if _, err := appInstance.GetPermManager().RevokeGroupPermission(req.TargetID, objType, req.ObjID, action, userInfo.GetTenantID()); err != nil {
						errJSON(c, http.StatusInternalServerError, err)
						return
					}
				}
			default:
				errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid target type"))
				return
			}
		}
		okJSON(c, gin.H{"revoked": true})
	}
}

func CreateGroup(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can create group"))
			return
		}

		req := groupRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if req.GroupID == "" {
			req.GroupID = uuid.New().String()
		}
		g := &models.UserGroup{
			GroupID:   req.GroupID,
			GroupName: req.GroupName,
			TenantID:  userInfo.GetTenantID(),
			Comment:   req.Comment,
		}
		if err := appInstance.GetDBManager().GetDefaultDB().Create(g).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, g)
	}
}

func DeleteGroup(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if appInstance.GetPermManager() == nil {
			errJSON(c, http.StatusInternalServerError, fmt.Errorf("permission manager is not initialized"))
			return
		}
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can delete group"))
			return
		}

		req := groupRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		db := appInstance.GetDBManager().GetDefaultDB()
		if err := db.Unscoped().Where(&models.UserGroup{GroupID: req.GroupID, TenantID: userInfo.GetTenantID()}).Delete(&models.UserGroup{}).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		if appInstance.GetEnforcer() != nil {
			_, _ = appInstance.GetEnforcer().RemoveFilteredGroupingPolicy(1, rbacsvc.GroupSubject(req.GroupID), rbacsvc.TenantDomain(userInfo.GetTenantID()))
			_, _ = appInstance.GetEnforcer().RemoveFilteredPolicy(0, rbacsvc.GroupSubject(req.GroupID), "", "", rbacsvc.TenantDomain(userInfo.GetTenantID()))
		}
		okJSON(c, gin.H{"deleted": true})
	}
}

func ListGroups(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		var groups []*models.UserGroup
		if err := appInstance.GetDBManager().GetDefaultDB().
			Where("tenant_id = ?", userInfo.GetTenantID()).Scopes(func(db *gorm.DB) *gorm.DB {
			if !userInfo.IsAdmin() {
				return db.Where("language_group_id = ? AND language_group_id <> ?", models.GroupID(userInfo), "")
			}
			return db
		}).
			Preload("Users").
			Find(&groups).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, sanitizeGroups(groups))
	}
}

func ListUsers(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !models.IsAccountManager(userInfo) {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can list users"))
			return
		}

		var users []*models.User
		if err := appInstance.GetDBManager().GetDefaultDB().
			Where(&models.User{UserEntity: &models.UserEntity{TenantID: userInfo.GetTenantID()}}).
			Scopes(func(db *gorm.DB) *gorm.DB {
				if !userInfo.IsAdmin() {
					return db.Where("language_group_id = ?", models.GroupID(userInfo))
				}
				return db
			}).
			Find(&users).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		ret := make([]models.UserEntity, 0, len(users))
		for _, user := range users {
			if user.UserEntity == nil {
				continue
			}
			ret = append(ret, user.GetSafeUserInfo())
		}
		okJSON(c, ret)
	}
}

func ListResourcePermissions(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can list resource permissions"))
			return
		}
		if appInstance.GetEnforcer() == nil {
			errJSON(c, http.StatusInternalServerError, fmt.Errorf("permission manager is not initialized"))
			return
		}

		req := resourcePermissionRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		objType := defs.RBACObj(req.ObjType)
		switch objType {
		case defs.RBACObjClient, defs.RBACObjServer:
		default:
			errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid object type"))
			return
		}
		if strings.TrimSpace(req.ObjID) == "" {
			errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid object id"))
			return
		}

		userNames, groupNames := loadPermissionSubjectNames(appInstance, userInfo.GetTenantID())
		domain := rbacsvc.TenantDomain(userInfo.GetTenantID())
		object := rbacsvc.Object(objType, req.ObjID)
		policies, err := appInstance.GetEnforcer().GetFilteredPolicy(1, object)
		if err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		merged := map[string]*resourcePermissionEntry{}
		for _, policy := range policies {
			if len(policy) < 4 || policy[3] != domain {
				continue
			}
			targetType, targetID := splitPermissionSubject(policy[0])
			if targetType == "" || targetID == "" {
				continue
			}
			key := targetType + ":" + targetID
			entry, ok := merged[key]
			if !ok {
				entry = &resourcePermissionEntry{
					TargetType: targetType,
					TargetID:   targetID,
					TargetName: targetID,
					Permission: string(defs.RBACActionView),
				}
				if targetType == string(defs.RBACSubjectUser) {
					if name := userNames[targetID]; name != "" {
						entry.TargetName = name
					}
				}
				if targetType == string(defs.RBACSubjectGroup) {
					if name := groupNames[targetID]; name != "" {
						entry.TargetName = name
					}
				}
				merged[key] = entry
			}
			if normalizePublicPermission(defs.RBACAction(policy[2])) == defs.RBACActionEdit {
				entry.Permission = string(defs.RBACActionEdit)
			}
		}

		ret := make([]resourcePermissionEntry, 0, len(merged))
		for _, entry := range merged {
			ret = append(ret, *entry)
		}
		sort.Slice(ret, func(i, j int) bool {
			if ret[i].TargetType == ret[j].TargetType {
				return ret[i].TargetName < ret[j].TargetName
			}
			return ret[i].TargetType < ret[j].TargetType
		})
		okJSON(c, ret)
	}
}

func UpdateUser(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !models.IsAccountManager(userInfo) {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can update users"))
			return
		}
		req := userAdminRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if req.UserID <= 0 {
			errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid user id"))
			return
		}
		if req.Role != "" && req.Role != defs.UserRole_Admin && req.Role != defs.UserRole_Normal && req.Role != defs.UserRole_GroupAdmin {
			errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid role"))
			return
		}

		models.NodeIdentityMu.Lock()
		defer models.NodeIdentityMu.Unlock()
		db := appInstance.GetDBManager().GetDefaultDB()
		user := &models.User{}
		if err := db.Where(&models.User{UserEntity: &models.UserEntity{UserID: req.UserID, TenantID: userInfo.GetTenantID()}}).First(user).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		if !userInfo.IsAdmin() && (user.Role != defs.UserRole_Normal || user.LanguageGroupID != models.GroupID(userInfo) || (req.Role != "" && req.Role != defs.UserRole_Normal)) {
			errJSON(c, 403, fmt.Errorf("只能管理本语系普通用户"))
			return
		}
		if user.UserID == userInfo.GetUserID() && ((req.Role != "" && req.Role != user.Role) || (req.Status != nil && *req.Status == models.STATUS_BANED)) {
			errJSON(c, 400, fmt.Errorf("不能撤销自己的管理权限"))
			return
		}
		if req.Role != "" && req.Role != defs.UserRole_Admin && user.LanguageGroupID == "" {
			errJSON(c, 400, fmt.Errorf("请先迁移到语系"))
			return
		}
		user.SessionVersion++
		if req.Role != "" {
			user.Role = req.Role
		}
		if req.UserName != "" {
			if err := utils.ValidateUserName(req.UserName); err != nil {
				errJSON(c, 400, err)
				return
			}
			user.UserName = req.UserName
		}
		if req.Email != "" {
			if err := utils.ValidateEmail(req.Email); err != nil {
				errJSON(c, 400, err)
				return
			}
			user.Email = req.Email
		}
		if req.Status != nil {
			if *req.Status != models.STATUS_NORMAL && *req.Status != models.STATUS_BANED {
				errJSON(c, 400, fmt.Errorf("invalid status"))
				return
			}
			user.Status = *req.Status
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(user).Error; err != nil {
				return err
			}
			return auditOrganization(tx, userInfo, user.LanguageGroupID, "user-update", fmt.Sprint(user.UserID), fmt.Sprintf("role=%s status=%d", user.Role, user.Status))
		}); err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, user.GetSafeUserInfo())
	}
}

func CreateInvite(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !models.IsAccountManager(userInfo) {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can create invite code"))
			return
		}
		req := inviteCreateRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if !userInfo.IsAdmin() {
			req.LanguageGroupID = models.GroupID(userInfo)
		}
		var group models.LanguageGroup
		if err := appInstance.GetDBManager().GetDefaultDB().Where("id = ? AND tenant_id = ?", req.LanguageGroupID, userInfo.GetTenantID()).First(&group).Error; err != nil {
			errJSON(c, 400, fmt.Errorf("请选择所属语系"))
			return
		}
		if req.Code == "" {
			req.Code = uuid.NewString()
		}
		if req.MaxUses <= 0 {
			req.MaxUses = 1
		}
		var expiresAt *time.Time
		if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
			t := time.Unix(*req.ExpiresAt, 0)
			expiresAt = &t
		}
		invite := &models.InviteCode{
			Code:            req.Code,
			LanguageGroupID: req.LanguageGroupID,
			TenantID:        userInfo.GetTenantID(),
			CreatedBy:       userInfo.GetUserID(),
			MaxUses:         req.MaxUses,
			ExpiresAt:       expiresAt,
			Comment:         req.Comment,
		}
		if err := appInstance.GetDBManager().GetDefaultDB().Create(invite).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, invite)
	}
}

func ListInvites(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !models.IsAccountManager(userInfo) {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can list invite codes"))
			return
		}
		now := time.Now()
		if err := appInstance.GetDBManager().GetDefaultDB().Scopes(func(db *gorm.DB) *gorm.DB {
			if !userInfo.IsAdmin() {
				return db.Where("language_group_id = ?", models.GroupID(userInfo))
			}
			return db
		}).
			Where("tenant_id = ? AND ((max_uses > 0 AND used_count >= max_uses) OR (expires_at IS NOT NULL AND expires_at <= ?))", userInfo.GetTenantID(), now).
			Unscoped().
			Delete(&models.InviteCode{}).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		var invites []*models.InviteCode
		if err := appInstance.GetDBManager().GetDefaultDB().Scopes(func(db *gorm.DB) *gorm.DB {
			if !userInfo.IsAdmin() {
				return db.Where("language_group_id = ?", models.GroupID(userInfo))
			}
			return db
		}).
			Where("tenant_id = ? AND (max_uses <= 0 OR used_count < max_uses) AND (expires_at IS NULL OR expires_at > ?)", userInfo.GetTenantID(), now).
			Order("created_at desc").
			Find(&invites).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, invites)
	}
}

func UpdateInvite(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !models.IsAccountManager(userInfo) {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can update invite code"))
			return
		}
		req := inviteUpdateRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		invite := &models.InviteCode{}
		db := appInstance.GetDBManager().GetDefaultDB()
		if err := db.Where(&models.InviteCode{ID: req.ID, TenantID: userInfo.GetTenantID()}).First(invite).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		if !userInfo.IsAdmin() && invite.LanguageGroupID != models.GroupID(userInfo) {
			errJSON(c, 403, fmt.Errorf("不能管理其他语系的邀请码"))
			return
		}
		if req.Disabled != nil {
			if *req.Disabled {
				if err := db.Unscoped().Delete(invite).Error; err != nil {
					errJSON(c, http.StatusInternalServerError, err)
					return
				}
				okJSON(c, gin.H{"deleted": true})
				return
			}
			invite.Disabled = *req.Disabled
		}
		if err := db.Save(invite).Error; err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, invite)
	}
}

func GetRegisterSetting(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		okJSON(c, gin.H{
			"register_enabled": getRegisterEnabled(appInstance),
			"invite_required":  getInviteRequired(appInstance),
		})
	}
}

func UpdateRegisterSetting(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can update register setting"))
			return
		}
		req := registerSettingRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		db := appInstance.GetDBManager().GetDefaultDB()

		if req.RegisterEnabled != nil {
			setting := &models.SystemSetting{
				Key:      authsvc.RegisterEnabledSettingKey,
				TenantID: 0,
				Value:    boolSettingValue(*req.RegisterEnabled),
			}
			if err := db.Save(setting).Error; err != nil {
				errJSON(c, http.StatusInternalServerError, err)
				return
			}
		}

		if req.InviteRequired != nil {
			setting := &models.SystemSetting{
				Key:      authsvc.RegisterInviteRequiredSettingKey,
				TenantID: 0,
				Value:    boolSettingValue(*req.InviteRequired),
			}
			if err := db.Save(setting).Error; err != nil {
				errJSON(c, http.StatusInternalServerError, err)
				return
			}
		}

		okJSON(c, gin.H{
			"register_enabled": getRegisterEnabled(appInstance),
			"invite_required":  getInviteRequired(appInstance),
		})
	}
}

func AddGroupMember(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if appInstance.GetPermManager() == nil {
			errJSON(c, http.StatusInternalServerError, fmt.Errorf("permission manager is not initialized"))
			return
		}
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can update group members"))
			return
		}
		req := groupMemberRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}

		db := appInstance.GetDBManager().GetDefaultDB()
		group := &models.UserGroup{}
		if err := db.Where(&models.UserGroup{GroupID: req.GroupID, TenantID: userInfo.GetTenantID()}).First(group).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		user := &models.User{}
		if err := db.Where(&models.User{UserEntity: &models.UserEntity{UserID: req.UserID, TenantID: userInfo.GetTenantID()}}).First(user).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		if err := db.Model(group).Association("Users").Append(user); err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		if _, err := appInstance.GetPermManager().AddUserToGroup(req.UserID, req.GroupID, userInfo.GetTenantID()); err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, gin.H{"added": true})
	}
}

func RemoveGroupMember(appInstance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := common.GetUserInfo(c)
		if !userInfo.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("only admin can update group members"))
			return
		}
		req := groupMemberRequest{}
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}

		db := appInstance.GetDBManager().GetDefaultDB()
		group := &models.UserGroup{}
		if err := db.Where(&models.UserGroup{GroupID: req.GroupID, TenantID: userInfo.GetTenantID()}).First(group).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		user := &models.User{}
		if err := db.Where(&models.User{UserEntity: &models.UserEntity{UserID: req.UserID, TenantID: userInfo.GetTenantID()}}).First(user).Error; err != nil {
			errJSON(c, http.StatusNotFound, err)
			return
		}
		if err := db.Model(group).Association("Users").Delete(user); err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		if _, err := appInstance.GetPermManager().RemoveUserFromGroup(req.UserID, req.GroupID, userInfo.GetTenantID()); err != nil {
			errJSON(c, http.StatusInternalServerError, err)
			return
		}
		okJSON(c, gin.H{"removed": true})
	}
}

func parsePermissionRequest(req permissionRequest) (defs.RBACObj, []defs.RBACAction, error) {
	if req.ObjID == "" || req.TargetType == "" || req.TargetID == "" {
		return "", nil, fmt.Errorf("invalid permission request")
	}

	objType := defs.RBACObj(req.ObjType)
	switch objType {
	case defs.RBACObjClient, defs.RBACObjServer, defs.RBACObjWorker:
	default:
		return "", nil, fmt.Errorf("invalid object type")
	}

	if req.Permission != "" {
		permission := defs.RBACAction(req.Permission)
		if permission != defs.RBACActionView && permission != defs.RBACActionEdit {
			return "", nil, fmt.Errorf("invalid permission: %s", req.Permission)
		}
		return objType, []defs.RBACAction{permission}, nil
	}

	actions := make([]defs.RBACAction, 0, len(req.Actions))
	for _, raw := range req.Actions {
		action := normalizePublicPermission(defs.RBACAction(raw))
		switch action {
		case defs.RBACActionView, defs.RBACActionEdit:
			actions = append(actions, action)
		default:
			return "", nil, fmt.Errorf("invalid permission: %s", raw)
		}
	}
	if len(actions) == 0 {
		return "", nil, fmt.Errorf("invalid permission request")
	}
	return objType, actions, nil
}

func canShare(ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, objID string) error {
	if userInfo == nil || !userInfo.Valid() {
		return fmt.Errorf("invalid user")
	}
	if userInfo.IsAdmin() {
		return nil
	}

	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	var ownerID int
	var tenantID int
	switch objType {
	case defs.RBACObjClient:
		item := &models.Client{}
		if err := db.Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: objID}}).First(item).Error; err != nil {
			return err
		}
		ownerID, tenantID = item.UserID, item.TenantID
	case defs.RBACObjServer:
		item := &models.Server{}
		if err := db.Where(&models.Server{ServerEntity: &models.ServerEntity{ServerID: objID}}).First(item).Error; err != nil {
			return err
		}
		ownerID, tenantID = item.UserID, item.TenantID
	case defs.RBACObjWorker:
		item := &models.Worker{}
		if err := db.Where(&models.Worker{WorkerEntity: &models.WorkerEntity{ID: objID}}).First(item).Error; err != nil {
			return err
		}
		ownerID, tenantID = int(item.UserId), int(item.TenantId)
	default:
		return fmt.Errorf("invalid object type")
	}
	if tenantID != userInfo.GetTenantID() {
		return fmt.Errorf("permission denied")
	}
	if dao.CanManageOwner(db, userInfo, ownerID) {
		return nil
	}
	return fmt.Errorf("只有设备主人或上级管理员可修改共享授权")
}

func okJSON(c *gin.Context, data any) {
	c.JSON(http.StatusOK, common.OK("ok").WithBody(data))
}

func errJSON(c *gin.Context, status int, err error) {
	c.JSON(status, common.Err(err.Error()))
}

func expandPermission(permission defs.RBACAction) []defs.RBACAction {
	if permission == defs.RBACActionEdit {
		return []defs.RBACAction{defs.RBACActionView, defs.RBACActionEdit}
	}
	return []defs.RBACAction{defs.RBACActionView}
}

func normalizePublicPermission(action defs.RBACAction) defs.RBACAction {
	switch action {
	case defs.RBACActionRead, defs.RBACActionView:
		return defs.RBACActionView
	case defs.RBACActionUpdate, defs.RBACActionDelete, defs.RBACActionShare, defs.RBACActionEdit:
		return defs.RBACActionEdit
	default:
		return action
	}
}

func ensureTenantUser(appInstance app.Application, tenantID int, userID int) error {
	user := &models.User{}
	return appInstance.GetDBManager().GetDefaultDB().
		Where(&models.User{UserEntity: &models.UserEntity{UserID: userID, TenantID: tenantID}}).
		First(user).Error
}

func ensureTenantGroup(appInstance app.Application, tenantID int, groupID string) error {
	group := &models.UserGroup{}
	return appInstance.GetDBManager().GetDefaultDB().
		Where(&models.UserGroup{GroupID: groupID, TenantID: tenantID}).
		First(group).Error
}

func loadPermissionSubjectNames(appInstance app.Application, tenantID int) (map[string]string, map[string]string) {
	userNames := map[string]string{}
	groupNames := map[string]string{}

	var users []*models.User
	if err := appInstance.GetDBManager().GetDefaultDB().
		Where(&models.User{UserEntity: &models.UserEntity{TenantID: tenantID}}).
		Find(&users).Error; err == nil {
		for _, user := range users {
			if user.UserEntity == nil {
				continue
			}
			name := user.UserName
			if name == "" {
				name = user.Email
			}
			userNames[fmt.Sprint(user.UserID)] = name
		}
	}

	var groups []*models.UserGroup
	if err := appInstance.GetDBManager().GetDefaultDB().
		Where(&models.UserGroup{TenantID: tenantID}).
		Find(&groups).Error; err == nil {
		for _, group := range groups {
			name := group.GroupName
			if name == "" {
				name = group.GroupID
			}
			groupNames[group.GroupID] = name
		}
	}
	return userNames, groupNames
}

func splitPermissionSubject(subject string) (string, string) {
	if strings.HasPrefix(subject, string(defs.RBACSubjectUser)+":") {
		return string(defs.RBACSubjectUser), strings.TrimPrefix(subject, string(defs.RBACSubjectUser)+":")
	}
	if strings.HasPrefix(subject, string(defs.RBACSubjectGroup)+":") {
		return string(defs.RBACSubjectGroup), strings.TrimPrefix(subject, string(defs.RBACSubjectGroup)+":")
	}
	return "", ""
}

func getInviteRequired(appInstance app.Application) bool {
	setting := &models.SystemSetting{}
	err := appInstance.GetDBManager().GetDefaultDB().
		Where(&models.SystemSetting{Key: authsvc.RegisterInviteRequiredSettingKey, TenantID: 0}).
		First(setting).Error
	if err != nil {
		return true
	}
	return setting.Value != "false"
}

func getRegisterEnabled(appInstance app.Application) bool {
	setting := &models.SystemSetting{}
	err := appInstance.GetDBManager().GetDefaultDB().
		Where(&models.SystemSetting{Key: authsvc.RegisterEnabledSettingKey, TenantID: 0}).
		First(setting).Error
	if err != nil {
		return appInstance.GetConfig().App.EnableRegister
	}
	return setting.Value == "true"
}

func boolSettingValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sanitizeGroups(groups []*models.UserGroup) []*models.UserGroup {
	for _, group := range groups {
		for _, user := range group.Users {
			if user.UserEntity == nil {
				continue
			}
			safe := user.GetSafeUserInfo()
			user.UserEntity = &safe
		}
	}
	return groups
}
