package platform

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var nodeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

func RenameNode(instance app.Application, kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID    string `json:"id"`
			NewID string `json:"newId"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(http.StatusBadRequest, common.Err("无效请求"))
			return
		}
		ctx := app.NewContext(c, instance)
		if err := renameNode(ctx, kind, req.ID, strings.TrimSpace(req.NewID)); err != nil {
			c.JSON(http.StatusBadRequest, common.Err(err.Error()))
			return
		}
		c.JSON(http.StatusOK, common.OK(defs.ReqSuccess).WithBody(gin.H{"id": strings.TrimSpace(req.NewID)}))
	}
}

func renameNode(ctx *app.Context, kind, oldID, newID string) error {
	models.NodeIdentityMu.Lock()
	defer models.NodeIdentityMu.Unlock()
	user := common.GetUserInfo(ctx)
	if !user.Valid() || (kind != "client" && kind != "server") {
		return fmt.Errorf("无权修改节点")
	}
	if !nodeIDPattern.MatchString(newID) {
		return fmt.Errorf("ID 需以字母或数字开头，仅支持字母、数字、点、横线和下划线，最多 128 字符")
	}
	if oldID == defs.DefaultServerID || newID == defs.DefaultServerID {
		return fmt.Errorf("不能修改或使用内置服务端 ID")
	}
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	err := db.Transaction(func(tx *gorm.DB) error {
		var owner, tenant int
		var client models.Client
		var server models.Server
		if kind == "client" {
			if err := tx.Where("client_id = ?", oldID).First(&client).Error; err != nil {
				return fmt.Errorf("客户端不存在")
			}
			if client.OriginClientID != "" {
				return fmt.Errorf("请修改物理客户端 ID，不能单独修改内部子配置 ID")
			}
			if client.Ephemeral {
				return fmt.Errorf("临时客户端不支持修改名称，请先接入为长期节点")
			}
			owner, tenant = client.UserID, client.TenantID
		} else {
			if err := tx.Where("server_id = ?", oldID).First(&server).Error; err != nil {
				return fmt.Errorf("服务端不存在")
			}
			owner, tenant = server.UserID, server.TenantID
		}
		if tenant != user.GetTenantID() || (!user.IsAdmin() && owner != user.GetUserID()) {
			return fmt.Errorf("只有节点所有者或管理员可以修改名称")
		}
		if oldID == newID {
			return nil
		}
		// Ownership comes from the database, never from a supplied prefix.
		var account models.User
		if err := tx.Where("user_id = ? AND tenant_id = ?", owner, tenant).First(&account).Error; err != nil {
			return fmt.Errorf("无法读取节点所属用户")
		}
		marker := ".c."
		if kind == "server" {
			marker = ".s."
		}
		prefix := account.UserName + marker
		if !strings.HasPrefix(newID, prefix) || len(newID) == len(prefix) {
			return fmt.Errorf("名称必须以 %s 开头，且后缀不能为空", prefix)
		}
		for _, target := range []struct{ table, column string }{{"clients", "client_id"}, {"servers", "server_id"}} {
			var count int64
			if err := tx.Table(target.table).Where(target.column+" = ?", newID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("该 ID 已存在，请使用其他 ID")
			}
		}
		// Create the replacement row first so references can migrate with FK checks on.
		if kind == "client" {
			copied := *client.ClientEntity
			copied.ClientID = newID
			if err := tx.Omit(clause.Associations).Create(&models.Client{ClientEntity: &copied}).Error; err != nil {
				return err
			}
		} else {
			copied := *server.ServerEntity
			copied.ServerID = newID
			if err := tx.Create(&models.Server{ServerEntity: &copied}).Error; err != nil {
				return err
			}
		}
		refs := []struct{ table, column string }{}
		if kind == "client" {
			refs = append(refs, struct{ table, column string }{"clients", "origin_client_id"}, struct{ table, column string }{"worker_clients", "client_client_id"}, struct{ table, column string }{"wireguards", "client_id"}, struct{ table, column string }{"endpoints", "client_id"})
			for _, table := range []string{"proxy_config", "proxy_stats", "history_proxy_stats"} {
				refs = append(refs, struct{ table, column string }{table, "client_id"}, struct{ table, column string }{table, "origin_client_id"})
			}
		} else {
			for _, table := range []string{"clients", "proxy_config", "proxy_stats", "history_proxy_stats"} {
				refs = append(refs, struct{ table, column string }{table, "server_id"})
			}
		}
		for _, ref := range refs {
			if !tx.Migrator().HasTable(ref.table) {
				continue
			}
			if err := tx.Table(ref.table).Where(ref.column+" = ?", oldID).Update(ref.column, newID).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasTable("casbin_rule") {
			if err := tx.Table("casbin_rule").Where("ptype = ? AND v1 = ? AND v3 = ?", "p", kind+":"+oldID, fmt.Sprintf("tenant:%d", tenant)).Update("v1", kind+":"+newID).Error; err != nil {
				return err
			}
		}

		if kind == "client" {
			return tx.Unscoped().Where("client_id = ?", oldID).Delete(&models.Client{}).Error
		}
		return tx.Unscoped().Where("server_id = ?", oldID).Delete(&models.Server{}).Error
	})
	if err != nil {
		return err
	}
	if oldID != newID {
		ctx.GetApp().GetClientsManager().Rename(oldID, newID)
		if pm := ctx.GetApp().GetPermManager(); pm != nil {
			if err := pm.Enforcer().LoadPolicy(); err != nil {
				return fmt.Errorf("名称已修改，但权限缓存刷新失败，请重启面板：%w", err)
			}
		}
	}
	return nil
}
