package rbac

import (
	"fmt"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/casbin/casbin/v2"
)

type permManager struct {
	enforcer *casbin.Enforcer
}

func (pm *permManager) Enforcer() *casbin.Enforcer {
	return pm.enforcer
}

func NewPermManager(enforcer *casbin.Enforcer) *permManager {
	return &permManager{
		enforcer: enforcer,
	}
}

func identity[T defs.RBACObj | defs.RBACSubject | defs.RBACDomain, U string | int | uint | int64](rType T, objID U) string {
	return string(rType) + ":" + fmt.Sprint(objID)
}

func UserSubject(userID int) string {
	return identity(defs.RBACSubjectUser, userID)
}

func GroupSubject(groupID string) string {
	return identity(defs.RBACSubjectGroup, groupID)
}

func Object(objType defs.RBACObj, objID string) string {
	return identity(objType, objID)
}

func TenantDomain(tenantID int) string {
	return identity(defs.RBACDomainTenant, tenantID)
}

func (pm *permManager) GrantGroupPermission(groupID string, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error) {
	groupSubject := GroupSubject(groupID)
	objSubject := Object(objType, objID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.AddPolicy(groupSubject, objSubject, string(action), domain)
}

func (pm *permManager) RevokeGroupPermission(groupID string, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error) {
	groupSubject := GroupSubject(groupID)
	objSubject := Object(objType, objID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.RemovePolicy(groupSubject, objSubject, string(action), domain)
}

func (pm *permManager) GrantUserPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error) {
	userSubject := UserSubject(userID)
	objSubject := Object(objType, objID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.AddPolicy(userSubject, objSubject, string(action), domain)
}

func (pm *permManager) RevokeUserPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error) {
	userSubject := UserSubject(userID)
	objSubject := Object(objType, objID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.RemovePolicy(userSubject, objSubject, string(action), domain)
}

func (pm *permManager) CheckPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error) {
	userSubject := UserSubject(userID)
	objSubject := Object(objType, objID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.Enforce(userSubject, objSubject, string(action), domain)
}

func (pm *permManager) AddUserToGroup(userID int, groupID string, tenantID int) (bool, error) {
	userSub := UserSubject(userID)
	groupSub := GroupSubject(groupID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.AddGroupingPolicy(userSub, groupSub, domain)
}

func (pm *permManager) RemoveUserFromGroup(userID int, groupID string, tenantID int) (bool, error) {
	userSub := UserSubject(userID)
	groupSub := GroupSubject(groupID)
	domain := TenantDomain(tenantID)

	return pm.enforcer.RemoveGroupingPolicy(userSub, groupSub, domain)
}
