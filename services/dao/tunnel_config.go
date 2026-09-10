package dao

import (
	"encoding/json"
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveTunnelConfig locks the server row so separate panel processes serialize
// port allocation, and commits the client config and proxy index together.
func SaveTunnelConfig(ctx *app.Context, u models.UserInfo, node *models.ClientEntity) error {
	if err := CanAccessClient(ctx, u, node.ClientID, defs.RBACActionEdit); err != nil {
		return err
	}
	return ctx.GetApp().GetDBManager().GetDefaultDB().Transaction(func(tx *gorm.DB) error {
		var server models.Server
		// A write is also a database-wide writer lock on SQLite.
		result := tx.Model(&models.Server{}).Where("server_id = ?", node.ServerID).UpdateColumn("server_id", node.ServerID)
		if result.Error != nil {
			return result.Error
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", node.ServerID).First(&server).Error; err != nil {
			return err
		}
		var owner models.User
		if err := tx.Where("user_id = ? AND tenant_id = ?", node.UserID, node.TenantID).First(&owner).Error; err != nil {
			return err
		}
		// A disabled panel account must not prevent an upper administrator managing its devices.
		owner.Status = models.STATUS_NORMAL
		if err := CanUseServer(tx, &owner, node.ServerID); err != nil {
			return err
		}
		configs, err := utils.LoadProxiesFromContent(node.ConfigContent)
		if err != nil {
			return err
		}
		var others []models.ProxyConfig
		if err = tx.Where("server_id = ? AND client_id <> ? AND stopped = ?", node.ServerID, node.ClientID, false).Find(&others).Error; err != nil {
			return err
		}
		occupied := map[string]bool{}
		for _, row := range others {
			key, err := proxyPortKey(row.Content)
			if err != nil {
				return err
			}
			if key != "" {
				occupied[key] = true
			}
		}
		names := map[string]bool{}
		rows := []*models.ProxyConfig{}
		for _, cfg := range configs {
			row := &models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{}}
			if err = row.FillClientConfig(node); err != nil {
				return err
			}
			if err = row.FillTypedProxyConfig(cfg); err != nil {
				return err
			}
			if names[row.Name] {
				return fmt.Errorf("duplicate proxy name")
			}
			names[row.Name] = true
			key, err := proxyPortKey(row.Content)
			if err != nil {
				return err
			}
			if key != "" {
				if occupied[key] {
					return fmt.Errorf("服务器端口已被占用：%s", key)
				}
				occupied[key] = true
			}
			var old models.ProxyConfig
			if err = tx.Where("client_id = ? AND name = ?", node.ClientID, row.Name).First(&old).Error; err == nil {
				row.Model = old.Model
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
			rows = append(rows, row)
		}
		if err = tx.Model(&models.Client{}).Where("client_id = ? AND tenant_id = ?", node.ClientID, node.TenantID).Updates(map[string]interface{}{"config_content": node.ConfigContent, "server_id": node.ServerID, "comment": node.Comment, "frps_url": node.FrpsUrl}).Error; err != nil {
			return err
		}
		if err = tx.Unscoped().Where("client_id = ? AND (stopped = ? OR stopped IS NULL)", node.ClientID, false).Delete(&models.ProxyConfig{}).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err = tx.Save(row).Error; err != nil {
				return err
			}
		}
		return tx.Create(&models.OrganizationAudit{TenantID: node.TenantID, LanguageGroupID: owner.LanguageGroupID, ActorID: u.GetUserID(), Action: "tunnel-config", Target: node.ClientID, Detail: fmt.Sprintf("owner=%d proxies=%d", node.UserID, len(rows))}).Error
	})
}
func proxyPortKey(content []byte) (string, error) {
	var cfg struct {
		Type       string `json:"type"`
		RemotePort int    `json:"remotePort"`
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return "", err
	}
	if cfg.Type != "tcp" && cfg.Type != "udp" {
		return "", nil
	}
	if cfg.RemotePort == 0 {
		return "", nil
	}
	if cfg.RemotePort < 1 || cfg.RemotePort > 65535 {
		return "", fmt.Errorf("invalid remote port")
	}
	return fmt.Sprintf("%s/%d", cfg.Type, cfg.RemotePort), nil
}
