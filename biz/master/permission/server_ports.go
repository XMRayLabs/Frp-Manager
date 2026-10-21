package permission

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

// Caller holds the same server row lock used by SaveTunnelConfig.
func saveServerPortAssignment(tx *gorm.DB, groupID, serverID string, requested *string, serverConfig []byte) error {
	grant := models.LanguageGroupServer{LanguageGroupID: groupID, ServerID: serverID}
	if requested == nil {
		// Old callers that only toggle authorization must not erase an existing restriction.
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&grant).Error
	}
	allowed, err := models.NormalizeAllowedPorts(*requested)
	if err != nil {
		return err
	}
	users := tx.Model(&models.User{}).Select("user_id").Where("language_group_id = ?", groupID)
	var proxies []models.ProxyConfig
	if err := tx.Where("user_id IN (?) AND server_id = ?", users, serverID).Find(&proxies).Error; err != nil {
		return err
	}
	conflicts := []string{}
	for _, proxy := range proxies {
		if err := models.ValidateProxyAllowedPorts(allowed, proxy.Content, serverConfig); err != nil {
			conflicts = append(conflicts, fmt.Sprintf("%s（%s）", proxy.Name, err))
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("无法保存：%d 条已有隧道超出新授权范围，请先修改或删除（含已停用隧道）：%s", len(conflicts), strings.Join(conflicts, "；"))
	}
	grant.AllowedPorts = allowed
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "language_group_id"}, {Name: "server_id"}}, DoUpdates: clause.AssignmentColumns([]string{"allowed_ports"})}).Create(&grant).Error
}

// A user moving into a restricted group must resolve incompatible saved tunnels first.
func validateMovingUserPorts(tx *gorm.DB, userID int, groupID string) error {
	var grants []models.LanguageGroupServer
	if err := tx.Where("language_group_id = ? AND allowed_ports <> ''", groupID).Find(&grants).Error; err != nil {
		return err
	}
	for _, grant := range grants {
		var server models.Server
		if err := tx.Where("server_id = ?", grant.ServerID).First(&server).Error; err != nil {
			return err
		}
		var proxies []models.ProxyConfig
		if err := tx.Where("user_id = ? AND server_id = ?", userID, grant.ServerID).Find(&proxies).Error; err != nil {
			return err
		}
		for _, proxy := range proxies {
			if err := models.ValidateProxyAllowedPorts(grant.AllowedPorts, proxy.Content, server.ConfigContent); err != nil {
				return fmt.Errorf("无法迁移：隧道 %s 不符合目标语系授权，请先修改或删除：%w", proxy.Name, err)
			}
		}
	}
	return nil
}
