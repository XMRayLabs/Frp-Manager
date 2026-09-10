package dao

import (
	"fmt"
	"strings"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	rbacsvc "github.com/Sakurame1/frp-manager/services/rbac"
	"gorm.io/gorm"
)

type ownedResource struct {
	tenantID int
	userID   int
}

func CanAccessClient(ctx *app.Context, userInfo models.UserInfo, clientID string, action defs.RBACAction) error {
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	client := &models.Client{}
	if err := db.Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: clientID}}).First(client).Error; err != nil {
		return err
	}
	err := canAccessResource(ctx, userInfo, defs.RBACObjClient, clientID, ownedResource{
		tenantID: client.TenantID,
		userID:   client.UserID,
	}, action)
	if err == nil || len(client.OriginClientID) == 0 {
		return err
	}
	return canAccessResource(ctx, userInfo, defs.RBACObjClient, client.OriginClientID, ownedResource{
		tenantID: client.TenantID,
		userID:   client.UserID,
	}, action)
}

func CanAccessServer(ctx *app.Context, userInfo models.UserInfo, serverID string, action defs.RBACAction) error {
	if action == defs.RBACActionView || action == defs.RBACActionRead {
		return CanUseServer(ctx.GetApp().GetDBManager().GetDefaultDB(), userInfo, serverID)
	}
	if !userInfo.IsAdmin() {
		return fmt.Errorf("仅网站管理员可管理服务端")
	}
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	server := &models.Server{}
	if err := db.Where(&models.Server{ServerEntity: &models.ServerEntity{ServerID: serverID}}).First(server).Error; err != nil {
		return err
	}
	return canAccessResource(ctx, userInfo, defs.RBACObjServer, serverID, ownedResource{
		tenantID: server.TenantID,
		userID:   server.UserID,
	}, action)
}

func scopeOwnedOrShared(db *gorm.DB, ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, idColumn string, action defs.RBACAction) *gorm.DB {
	return organizationScope(db, ctx, userInfo, objType, idColumn, action)
}
func scopeOwnedOrSharedUint(db *gorm.DB, ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, idColumn string, action defs.RBACAction) *gorm.DB {
	return organizationScope(db, ctx, userInfo, objType, idColumn, action)
}
func scopeUsableServers(db *gorm.DB, u models.UserInfo) *gorm.DB {
	if u == nil || !u.Valid() {
		return db.Where("1 = 0")
	}
	if u.IsAdmin() {
		return db.Where("tenant_id = ? OR server_id = ?", u.GetTenantID(), defs.DefaultServerID)
	}
	assignments := db.Session(&gorm.Session{NewDB: true}).Model(&models.LanguageGroupServer{}).Select("server_id").Where("language_group_id = ? AND language_group_id <> ''", models.GroupID(u))
	return db.Where("server_id IN (?)", assignments).Where("tenant_id = ? OR server_id = ?", u.GetTenantID(), defs.DefaultServerID)
}

func canAccessResource(ctx *app.Context, u models.UserInfo, obj defs.RBACObj, id string, res ownedResource, action defs.RBACAction) error {
	if u == nil || !u.Valid() || res.tenantID != u.GetTenantID() {
		return fmt.Errorf("permission denied")
	}
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	if obj == defs.RBACObjServer {
		if u.IsAdmin() {
			return nil
		}
		if action == defs.RBACActionView || action == defs.RBACActionRead {
			return CanUseServer(db, u, id)
		}
		return fmt.Errorf("仅网站管理员可管理服务端")
	}
	if CanManageOwner(db, u, res.userID) {
		return nil
	}
	if obj != defs.RBACObjClient || action == defs.RBACActionDelete || action == defs.RBACActionShare {
		return fmt.Errorf("permission denied")
	}
	var node models.Client
	if err := db.Where("client_id = ?", id).First(&node).Error; err != nil {
		return err
	}
	if node.OriginClientID != "" {
		var parent models.Client
		if err := db.Where("client_id = ?", node.OriginClientID).First(&parent).Error; err != nil {
			return err
		}
		node = parent
	}
	var owner models.User
	if err := db.Where("user_id = ?", res.userID).First(&owner).Error; err != nil {
		return err
	}
	if models.GroupID(u) == "" || owner.LanguageGroupID != models.GroupID(u) || node.Private {
		return fmt.Errorf("permission denied")
	}
	if groupShared(db, u) {
		return nil
	}
	if ctx.GetApp().GetPermManager() != nil {
		if ok, err := checkPermission(ctx, u, obj, id, action); err == nil && ok {
			return nil
		}
	}
	return fmt.Errorf("permission denied")
}

func grantOwnerPermissions(ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, objID string) {
	if ctx.GetApp().GetPermManager() == nil {
		return
	}

	for _, action := range []defs.RBACAction{defs.RBACActionView, defs.RBACActionEdit} {
		_, _ = ctx.GetApp().GetPermManager().GrantUserPermission(userInfo.GetUserID(), objType, objID, action, userInfo.GetTenantID())
	}
}

func revokeResourcePermissions(ctx *app.Context, objType defs.RBACObj, objID string, tenantID int) {
	enforcer := ctx.GetApp().GetEnforcer()
	if enforcer == nil {
		return
	}
	_, _ = enforcer.RemoveFilteredPolicy(1, rbacsvc.Object(objType, objID), "", rbacsvc.TenantDomain(tenantID))
}

func accessibleObjectIDs(ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, action defs.RBACAction) []string {
	enforcer := ctx.GetApp().GetEnforcer()
	if enforcer == nil {
		return nil
	}

	domain := rbacsvc.TenantDomain(userInfo.GetTenantID())
	actions := []defs.RBACAction{normalizeAction(action)}
	if normalizeAction(action) == defs.RBACActionView {
		actions = append(actions, defs.RBACActionEdit)
	}

	subjects := []string{rbacsvc.UserSubject(userInfo.GetUserID())}
	subjects = append(subjects, enforcer.GetRolesForUserInDomain(rbacsvc.UserSubject(userInfo.GetUserID()), domain)...)

	seen := map[string]struct{}{}
	var ids []string
	for _, subject := range subjects {
		for _, allowedAction := range actions {
			policies, _ := enforcer.GetFilteredPolicy(0, subject, "", string(allowedAction), domain)
			for _, policy := range policies {
				if len(policy) < 4 {
					continue
				}
				prefix := string(objType) + ":"
				if !strings.HasPrefix(policy[1], prefix) {
					continue
				}
				id := strings.TrimPrefix(policy[1], prefix)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func checkPermission(ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, objID string, action defs.RBACAction) (bool, error) {
	action = normalizeAction(action)
	if action == defs.RBACActionView {
		ok, err := ctx.GetApp().GetPermManager().CheckPermission(userInfo.GetUserID(), objType, objID, defs.RBACActionEdit, userInfo.GetTenantID())
		if err != nil || ok {
			return ok, err
		}
	}
	return ctx.GetApp().GetPermManager().CheckPermission(userInfo.GetUserID(), objType, objID, action, userInfo.GetTenantID())
}

func normalizeAction(action defs.RBACAction) defs.RBACAction {
	switch action {
	case defs.RBACActionRead:
		return defs.RBACActionView
	case defs.RBACActionUpdate, defs.RBACActionDelete, defs.RBACActionShare:
		return defs.RBACActionEdit
	default:
		return action
	}
}
