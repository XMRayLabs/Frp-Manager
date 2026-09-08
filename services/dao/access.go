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
	if serverID == defs.DefaultServerID && userInfo.Valid() {
		return nil
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
	if userInfo.IsAdmin() {
		return db.Where("tenant_id = ?", userInfo.GetTenantID())
	}

	sharedIDs := accessibleObjectIDs(ctx, userInfo, objType, action)
	scope := db.Where("tenant_id = ? AND user_id = ?", userInfo.GetTenantID(), userInfo.GetUserID())
	if len(sharedIDs) == 0 {
		return scope
	}

	return db.Where(
		db.Where("tenant_id = ? AND user_id = ?", userInfo.GetTenantID(), userInfo.GetUserID()).
			Or(fmt.Sprintf("tenant_id = ? AND %s IN ?", idColumn), userInfo.GetTenantID(), sharedIDs),
	)
}

func scopeOwnedOrSharedUint(db *gorm.DB, ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, idColumn string, action defs.RBACAction) *gorm.DB {
	if userInfo.IsAdmin() {
		return db.Where("tenant_id = ?", userInfo.GetTenantID())
	}

	sharedIDs := accessibleObjectIDs(ctx, userInfo, objType, action)
	scope := db.Where("tenant_id = ? AND user_id = ?", uint32(userInfo.GetTenantID()), uint32(userInfo.GetUserID()))
	if len(sharedIDs) == 0 {
		return scope
	}

	return db.Where(
		db.Where("tenant_id = ? AND user_id = ?", uint32(userInfo.GetTenantID()), uint32(userInfo.GetUserID())).
			Or(fmt.Sprintf("tenant_id = ? AND %s IN ?", idColumn), uint32(userInfo.GetTenantID()), sharedIDs),
	)
}

func scopeUsableServers(db *gorm.DB, userInfo models.UserInfo) *gorm.DB {
	return db.Where(
		db.Where("tenant_id = ?", userInfo.GetTenantID()).
			Or("server_id = ?", defs.DefaultServerID),
	)
}

func canAccessResource(ctx *app.Context, userInfo models.UserInfo, objType defs.RBACObj, objID string, res ownedResource, action defs.RBACAction) error {
	if res.tenantID != userInfo.GetTenantID() {
		return fmt.Errorf("permission denied")
	}
	if userInfo.IsAdmin() || res.userID == userInfo.GetUserID() {
		return nil
	}
	if ctx.GetApp().GetPermManager() == nil {
		return fmt.Errorf("permission denied")
	}

	ok, err := checkPermission(ctx, userInfo, objType, objID, action)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("permission denied")
	}
	return nil
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
