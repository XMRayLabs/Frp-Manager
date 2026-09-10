package dao

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"gorm.io/gorm"
)

func CanManageOwner(db *gorm.DB, u models.UserInfo, owner int) bool {
	if u == nil || !u.Valid() {
		return false
	}
	if u.IsAdmin() {
		return true
	}
	if owner == u.GetUserID() {
		return true
	}
	if !models.IsGroupAdmin(u) {
		return false
	}
	var count int64
	db.Model(&models.User{}).Where("user_id = ? AND tenant_id = ? AND language_group_id = ?", owner, u.GetTenantID(), models.GroupID(u)).Count(&count)
	return count == 1
}
func CanManageClient(ctx *app.Context, u models.UserInfo, id string) error {
	node, err := NewQuery(ctx).AdminGetClientByClientID(id)
	if err != nil {
		return err
	}
	if node.TenantID != u.GetTenantID() || !CanManageOwner(ctx.GetApp().GetDBManager().GetDefaultDB(), u, node.UserID) {
		return fmt.Errorf("仅设备所有者或上级管理员可执行此操作")
	}
	return nil
}
func CanUseServer(db *gorm.DB, u models.UserInfo, id string) error {
	if u == nil || !u.Valid() {
		return fmt.Errorf("permission denied")
	}
	var count int64
	if err := scopeUsableServers(db.Model(&models.Server{}), u).Where("server_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("当前语系未获此服务端授权")
	}
	return nil
}
func groupOwnerIDs(db *gorm.DB, u models.UserInfo) *gorm.DB {
	return db.Session(&gorm.Session{NewDB: true}).Model(&models.User{}).Select("user_id").Where("tenant_id = ? AND language_group_id = ? AND language_group_id <> ''", u.GetTenantID(), models.GroupID(u))
}
func groupShared(db *gorm.DB, u models.UserInfo) bool {
	if models.GroupID(u) == "" {
		return false
	}
	var count int64
	db.Model(&models.LanguageGroup{}).Where("id = ? AND tenant_id = ? AND shared = ?", models.GroupID(u), u.GetTenantID(), true).Count(&count)
	return count == 1
}
func organizationScope(db *gorm.DB, ctx *app.Context, u models.UserInfo, obj defs.RBACObj, column string, action defs.RBACAction) *gorm.DB {
	if u == nil || !u.Valid() {
		return db.Where("1 = 0")
	}
	if u.IsAdmin() {
		return db.Where("tenant_id = ?", u.GetTenantID())
	}
	base := db.Session(&gorm.Session{NewDB: true})
	if models.IsGroupAdmin(u) {
		return db.Where("tenant_id = ? AND user_id IN (?)", u.GetTenantID(), groupOwnerIDs(base, u))
	}
	owner := base.Where("user_id = ?", u.GetUserID())
	if models.GroupID(u) == "" {
		return db.Where("tenant_id = ? AND user_id = ?", u.GetTenantID(), u.GetUserID())
	}
	shared := base.Where("1 = 0")
	if obj == defs.RBACObjClient {
		nodes := base.Model(&models.Client{}).Select("client_id").Where("tenant_id = ? AND user_id IN (?) AND private = ?", u.GetTenantID(), groupOwnerIDs(base, u), false).Where("origin_client_id IS NULL OR origin_client_id = ''")
		allowed := base.Where("1 = 0")
		if groupShared(base, u) {
			allowed = base.Where("user_id IN (?)", groupOwnerIDs(base, u))
		}
		ids := accessibleObjectIDs(ctx, u, obj, action)
		if len(ids) > 0 {
			allowed = allowed.Or(column+" IN ?", ids)
		}
		// Client-scoped resources (including proxies) inherit the physical client's privacy.
		privacy := base.Where(column+" IN (?)", nodes)
		if column == "client_id" {
			privacy = privacy.Or("origin_client_id IN (?)", nodes)
		}
		shared = base.Where("user_id IN (?)", groupOwnerIDs(base, u)).Where(allowed).Where(privacy)
	}
	return db.Where("tenant_id = ?", u.GetTenantID()).Where(owner.Or(shared))
}

// Advanced resources are never shared implicitly with ordinary members.
func accountScope(db *gorm.DB, u models.UserInfo) *gorm.DB {
	if u == nil || !u.Valid() {
		return db.Where("1 = 0")
	}
	if u.IsAdmin() {
		return db.Where("tenant_id = ?", u.GetTenantID())
	}
	if models.IsGroupAdmin(u) {
		return db.Where("tenant_id = ? AND user_id IN (?)", u.GetTenantID(), groupOwnerIDs(db, u))
	}
	return db.Where("tenant_id = ? AND user_id = ?", u.GetTenantID(), u.GetUserID())
}
func endpointScope(db *gorm.DB, u models.UserInfo) *gorm.DB {
	nodes := accountScope(db.Session(&gorm.Session{NewDB: true}).Model(&models.Client{}), u).Select("client_id")
	return db.Where("client_id IN (?)", nodes)
}

func validateLinkTargets(ctx *app.Context, u models.UserInfo, link *models.WireGuardLink) error {
	if link == nil || link.WireGuardLinkEntity == nil {
		return fmt.Errorf("invalid link")
	}
	q := NewQuery(ctx)
	from, err := q.GetWireGuardByID(u, link.FromWireGuardID)
	if err != nil {
		return err
	}
	to, err := q.GetWireGuardByID(u, link.ToWireGuardID)
	if err != nil {
		return err
	}
	if link.ToEndpointID != 0 {
		endpoint, err := q.GetEndpointByID(u, link.ToEndpointID)
		if err != nil {
			return err
		}
		if endpoint.ClientID != to.ClientID {
			return fmt.Errorf("endpoint belongs to another device")
		}
	}
	if from.NetworkID != link.NetworkID || to.NetworkID != link.NetworkID {
		return fmt.Errorf("network mismatch")
	}
	if _, err := q.GetNetworkByID(u, link.NetworkID); err != nil {
		return err
	}
	return nil
}
