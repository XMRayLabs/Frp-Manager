package permission

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"
	"time"
)

// Organization operations always resolve role and membership from the authenticated database user.
func OrganizationRoutes(r *gin.RouterGroup, a app.Application) {
	r.GET("/context", func(c *gin.Context) { u := common.GetUserInfo(c); okJSON(c, u.GetSafeUserInfo()) })
	r.GET("/clients", organizationClients(a))
	r.GET("/groups", organizationHandler(a, "list"))
	r.POST("/groups/save", organizationHandler(a, "save"))
	r.POST("/groups/delete", organizationHandler(a, "delete"))
	r.POST("/servers/assign", organizationHandler(a, "assign"))
	r.POST("/users/move", organizationHandler(a, "move"))
	r.POST("/users/create-admin", organizationHandler(a, "create-admin"))
	r.POST("/users/reset-password", organizationHandler(a, "reset-password"))
	r.POST("/clients/privacy", organizationHandler(a, "privacy"))
	r.GET("/audit", organizationHandler(a, "audit"))
}

type organizationRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Shared   *bool  `json:"shared"`
	UserID   int    `json:"user_id"`
	ServerID string `json:"server_id"`
	Assigned bool   `json:"assigned"`
	Confirm  bool   `json:"confirm"`
	ClientID string `json:"client_id"`
	Private  bool   `json:"private"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func organizationHandler(a app.Application, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := common.GetUserInfo(c)
		db := a.GetDBManager().GetDefaultDB()
		ctx := app.NewContext(c, a)
		if u == nil || !u.Valid() {
			errJSON(c, 403, fmt.Errorf("permission denied"))
			return
		}
		req := organizationRequest{}
		if c.Request.Method == "POST" {
			if c.ShouldBindJSON(&req) != nil {
				errJSON(c, 400, fmt.Errorf("invalid request"))
				return
			}
		}
		if action == "privacy" {
			if err := dao.CanManageClient(ctx, u, req.ClientID); err != nil {
				errJSON(c, 403, err)
				return
			}
			var node models.Client
			if err := db.Where("client_id = ? AND (origin_client_id IS NULL OR origin_client_id = '')", req.ClientID).First(&node).Error; err != nil {
				errJSON(c, 400, err)
				return
			}
			err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.Client{}).Where("client_id = ? OR origin_client_id = ?", req.ClientID, req.ClientID).Update("private", req.Private).Error; err != nil {
					return err
				}
				return auditOrganization(tx, u, models.GroupID(u), action, req.ClientID, fmt.Sprint(req.Private))
			})
			if err != nil {
				errJSON(c, 400, err)
				return
			}
			okJSON(c, gin.H{"private": req.Private})
			return
		}
		if !models.IsAccountManager(u) {
			errJSON(c, 403, fmt.Errorf("仅管理员可管理语系"))
			return
		}
		if action == "list" {
			var groups []models.LanguageGroup
			q := db.Where("tenant_id = ?", u.GetTenantID())
			if !u.IsAdmin() {
				q = q.Where("id = ?", models.GroupID(u))
			}
			if err := q.Order("name").Find(&groups).Error; err != nil {
				errJSON(c, 500, err)
				return
			}
			ids := []string{}
			for _, g := range groups {
				ids = append(ids, g.ID)
			}
			var grants []models.LanguageGroupServer
			if err := db.Where("language_group_id IN ?", ids).Find(&grants).Error; err != nil {
				errJSON(c, 500, err)
				return
			}
			okJSON(c, gin.H{"groups": groups, "assignments": grants})
			return
		}
		if action == "audit" {
			var rows []models.OrganizationAudit
			q := db.Where("tenant_id = ?", u.GetTenantID())
			if !u.IsAdmin() {
				q = q.Where("language_group_id = ?", models.GroupID(u))
			}
			if err := q.Order("id DESC").Limit(100).Find(&rows).Error; err != nil {
				errJSON(c, 500, err)
				return
			}
			okJSON(c, rows)
			return
		}
		if action == "reset-password" {
			var account models.User
			if err := db.Where("user_id = ? AND tenant_id = ?", req.UserID, u.GetTenantID()).First(&account).Error; err != nil {
				errJSON(c, 404, err)
				return
			}
			if !u.IsAdmin() && (account.Role != defs.UserRole_Normal || account.LanguageGroupID != models.GroupID(u)) {
				errJSON(c, 403, fmt.Errorf("不能管理此账号"))
				return
			}
			if !req.Confirm {
				okJSON(c, gin.H{"confirm_required": true, "initial_password": "邮箱地址", "user_id": account.UserID})
				return
			}
			hashed, err := utils.HashPassword(account.Email)
			if err != nil {
				errJSON(c, 400, err)
				return
			}
			err = db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.User{}).Where("user_id = ?", account.UserID).Updates(map[string]interface{}{"password": hashed, "must_change_password": true, "session_version": gorm.Expr("session_version + 1")}).Error; err != nil {
					return err
				}
				return auditOrganization(tx, u, account.LanguageGroupID, action, fmt.Sprint(account.UserID), "")
			})
			if err != nil {
				errJSON(c, 400, err)
				return
			}
			okJSON(c, gin.H{"ok": true})
			return
		}
		if !u.IsAdmin() && (action != "save" || req.ID != models.GroupID(u)) {
			errJSON(c, 403, fmt.Errorf("仅网站管理员可执行此操作"))
			return
		}
		if action != "save" && req.ID == "" {
			errJSON(c, 400, fmt.Errorf("请选择语系"))
			return
		}
		var group models.LanguageGroup
		if req.ID != "" {
			if err := db.Where("id = ? AND tenant_id = ?", req.ID, u.GetTenantID()).First(&group).Error; err != nil {
				errJSON(c, 404, err)
				return
			}
		}
		var result interface{}
		models.NodeIdentityMu.Lock()
		err := db.Transaction(func(tx *gorm.DB) error {
			switch action {
			case "save":
				if req.ID == "" {
					group = models.LanguageGroup{ID: uuid.NewString(), TenantID: u.GetTenantID()}
				}
				if u.IsAdmin() {
					group.Name = strings.TrimSpace(req.Name)
				}
				if group.Name == "" || len(group.Name) > 128 {
					return fmt.Errorf("语系名称需为 1–128 字节")
				}
				if req.Shared != nil {
					group.Shared = *req.Shared
				}
				if err := tx.Save(&group).Error; err != nil {
					return err
				}
				result = group
			case "delete":
				var count int64
				if err := tx.Model(&models.User{}).Where("language_group_id = ?", group.ID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					if err := tx.Model(&models.User{}).Unscoped().Where("language_group_id = ?", group.ID).Count(&count).Error; err != nil {
						return err
					}
				}
				if count > 0 {
					return fmt.Errorf("语系仍有成员，请先迁移")
				}
				if err := tx.Where("language_group_id = ?", group.ID).Delete(&models.LanguageGroupServer{}).Error; err != nil {
					return err
				}
				if err := tx.Where("language_group_id = ?", group.ID).Delete(&models.InviteCode{}).Error; err != nil {
					return err
				}
				if err := tx.Delete(&group).Error; err != nil {
					return err
				}
			case "create-admin":
				if group.ID == "" {
					return fmt.Errorf("请选择语系")
				}
				if err := utils.ValidateUserName(req.Username); err != nil {
					return err
				}
				if err := utils.ValidateEmail(req.Email); err != nil {
					return err
				}
				password, err := utils.HashPassword(req.Email)
				if err != nil {
					return err
				}
				account := &models.User{UserEntity: &models.UserEntity{UserName: req.Username, Email: req.Email, Password: password, Role: defs.UserRole_GroupAdmin, LanguageGroupID: group.ID, TenantID: u.GetTenantID(), Token: uuid.NewString(), Status: models.STATUS_NORMAL, MustChangePassword: true}}
				if err = tx.Create(account).Error; err != nil {
					return fmt.Errorf("创建失败，用户名或邮箱可能已存在")
				}
				result = account.GetSafeUserInfo()
			case "assign":
				var server models.Server
				if err := tx.Model(&models.Server{}).Where("server_id = ?", req.ServerID).UpdateColumn("server_id", req.ServerID).Error; err != nil {
					return err
				}
				if err := tx.Where("server_id = ? AND (tenant_id = ? OR server_id = ?)", req.ServerID, u.GetTenantID(), defs.DefaultServerID).First(&server).Error; err != nil {
					return err
				}
				if group.ID == "" {
					return fmt.Errorf("请选择语系")
				}
				if req.Assigned {
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.LanguageGroupServer{LanguageGroupID: group.ID, ServerID: req.ServerID}).Error; err != nil {
						return err
					}
				} else {
					users := tx.Model(&models.User{}).Select("user_id").Where("language_group_id = ?", group.ID)
					var affected []models.ProxyConfig
					if err := tx.Where("user_id IN (?) AND server_id = ?", users, req.ServerID).Find(&affected).Error; err != nil {
						return err
					}
					result = gin.H{"affected_count": len(affected), "affected": proxyImpact(affected), "confirm_required": !req.Confirm}
					if !req.Confirm {
						return nil
					}
					if err := suspendOrganizationTunnels(tx, users, []string{req.ServerID}); err != nil {
						return err
					}
					if err := tx.Where("language_group_id = ? AND server_id = ?", group.ID, req.ServerID).Delete(&models.LanguageGroupServer{}).Error; err != nil {
						return err
					}
				}
			case "move":
				if group.ID == "" {
					return fmt.Errorf("请选择目标语系")
				}
				var account models.User
				if err := tx.Where("user_id = ? AND tenant_id = ? AND role <> ?", req.UserID, u.GetTenantID(), defs.UserRole_Admin).First(&account).Error; err != nil {
					return err
				}
				grants := tx.Model(&models.LanguageGroupServer{}).Select("server_id").Where("language_group_id = ?", group.ID)
				var affected []models.ProxyConfig
				if err := tx.Where("user_id = ? AND server_id NOT IN (?)", req.UserID, grants).Find(&affected).Error; err != nil {
					return err
				}
				result = gin.H{"affected_count": len(affected), "affected": proxyImpact(affected), "confirm_required": !req.Confirm}
				if !req.Confirm {
					return nil
				}
				serverIDs := []string{}
				for _, p := range affected {
					serverIDs = append(serverIDs, p.ServerID)
				}
				users := tx.Model(&models.User{}).Select("user_id").Where("user_id = ?", req.UserID)
				if err := suspendOrganizationTunnels(tx, users, serverIDs); err != nil {
					return err
				}
				if err := tx.Where("created_by = ?", req.UserID).Delete(&models.InviteCode{}).Error; err != nil {
					return err
				}
				if tx.Migrator().HasTable("casbin_rule") {
					if err := tx.Exec("DELETE FROM casbin_rule WHERE v0 = ?", fmt.Sprintf("user:%d", req.UserID)).Error; err != nil {
						return err
					}
				}
				if err := tx.Model(&account).Updates(map[string]interface{}{"language_group_id": group.ID, "session_version": gorm.Expr("session_version + 1")}).Error; err != nil {
					return err
				}
				if tx.Migrator().HasTable("user_group_memberships") {
					if err := tx.Table("user_group_memberships").Where("user_user_id = ?", req.UserID).Delete(map[string]interface{}{}).Error; err != nil {
						return err
					}
				}
			default:
				return fmt.Errorf("unsupported operation")
			}
			if (action == "move" || action == "assign" && !req.Assigned) && !req.Confirm {
				return nil
			}
			return auditOrganization(tx, u, group.ID, action, req.ID, fmt.Sprintf("user=%d server=%s", req.UserID, req.ServerID))
		})
		models.NodeIdentityMu.Unlock()
		if err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		if action == "move" && req.Confirm && a.GetEnforcer() != nil {
			if err := a.GetEnforcer().LoadPolicy(); err != nil {
				errJSON(c, 500, fmt.Errorf("迁移已保存，权限缓存刷新失败，请重启面板: %w", err))
				return
			}
		}
		if req.Confirm && (action == "move" || action == "assign" && !req.Assigned) {
			var stopped []models.Client
			scope := db.Where("stopped = ? AND tenant_id = ?", true, u.GetTenantID())
			if action == "move" {
				scope = scope.Where("user_id = ?", req.UserID)
			} else {
				scope = scope.Where("server_id = ? AND user_id IN (?)", req.ServerID, db.Model(&models.User{}).Select("user_id").Where("language_group_id = ?", group.ID))
			}
			if err := scope.Find(&stopped).Error; err == nil {
				go func() {
					for _, node := range stopped {
						destination := node.OriginClientID
						if destination == "" {
							destination = node.ClientID
						}
						cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						id := node.ClientID
						_, _ = rpc.CallClient(app.NewContext(cancelCtx, a), destination, pb.Event_EVENT_STOP_FRPC, &pb.StopFRPCRequest{ClientId: &id})
						cancel()
					}
				}()
			}
		}
		okJSON(c, result)
	}
}
func auditOrganization(tx *gorm.DB, u models.UserInfo, group, action, target, detail string) error {
	return tx.Create(&models.OrganizationAudit{TenantID: u.GetTenantID(), LanguageGroupID: group, ActorID: u.GetUserID(), Action: action, Target: target, Detail: detail}).Error
}
func proxyImpact(rows []models.ProxyConfig) []gin.H {
	out := []gin.H{}
	for _, p := range rows {
		out = append(out, gin.H{"id": p.ID, "name": p.Name, "client_id": p.OriginClientID, "server_id": p.ServerID})
	}
	return out
}
func suspendOrganizationTunnels(tx *gorm.DB, users *gorm.DB, servers []string) error {
	if len(servers) == 0 {
		return nil
	}
	if err := tx.Model(&models.ProxyConfig{}).Where("user_id IN (?) AND server_id IN ?", users, servers).Update("stopped", true).Error; err != nil {
		return err
	}
	var clients []models.Client
	if err := tx.Where("user_id IN (?) AND server_id IN ?", users, servers).Find(&clients).Error; err != nil {
		return err
	}
	for _, node := range clients {
		var cfg map[string]interface{}
		if len(node.ConfigContent) > 0 {
			if err := json.Unmarshal(node.ConfigContent, &cfg); err != nil {
				return err
			}
			if cfg == nil {
				cfg = map[string]interface{}{}
			}
			cfg["proxies"] = []interface{}{}
			cfg["visitors"] = []interface{}{}
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.Client{}).Where("client_id = ?", node.ClientID).Updates(map[string]interface{}{"stopped": true, "config_content": data}).Error; err != nil {
			return err
		}
	}
	return nil
}

func organizationClients(a app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := common.GetUserInfo(c)
		ctx := app.NewContext(c, a)
		rows, err := dao.NewQuery(ctx).GetAllClients(u)
		if err != nil {
			errJSON(c, 500, err)
			return
		}
		ids := []int{}
		for _, node := range rows {
			ids = append(ids, node.UserID)
		}
		var owners []models.User
		if err := a.GetDBManager().GetDefaultDB().Select("user_id,user_name,language_group_id").Where("user_id IN ?", ids).Find(&owners).Error; err != nil {
			errJSON(c, 500, err)
			return
		}
		byID := map[int]models.User{}
		for _, owner := range owners {
			byID[owner.UserID] = owner
		}
		result := map[string]gin.H{}
		for _, node := range rows {
			owner, ok := byID[node.UserID]
			if !ok {
				continue
			}
			result[node.ClientID] = gin.H{"private": node.Private, "mine": node.UserID == u.GetUserID(), "owner_name": owner.UserName, "group_id": owner.LanguageGroupID}
		}
		okJSON(c, result)
	}
}
