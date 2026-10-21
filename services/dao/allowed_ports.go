package dao

import (
	"encoding/json"
	"github.com/Sakurame1/frp-manager/models"
	"gorm.io/gorm"
)

// Resolve the device owner's grant, never the administrator acting on its behalf.
func ServerAllowedPorts(db *gorm.DB, owner models.UserInfo, serverID string) (string, error) {
	if err := CanUseServer(db, owner, serverID); err != nil {
		return "", err
	}
	if owner.IsAdmin() {
		return "", nil
	}
	var grant models.LanguageGroupServer
	if err := db.Where("language_group_id = ? AND server_id = ?", models.GroupID(owner), serverID).First(&grant).Error; err != nil {
		return "", err
	}
	return models.NormalizeAllowedPorts(grant.AllowedPorts)
}

func ValidateClientAllowedPorts(db *gorm.DB, owner models.UserInfo, node *models.ClientEntity) error {
	allowed, err := ServerAllowedPorts(db, owner, node.ServerID)
	if err != nil {
		return err
	}
	if allowed == "" {
		return nil
	}
	var server models.Server
	if err := db.Where("server_id = ?", node.ServerID).First(&server).Error; err != nil {
		return err
	}
	var config struct {
		Proxies []json.RawMessage `json:"proxies"`
	}
	if err := json.Unmarshal(node.ConfigContent, &config); err != nil {
		return err
	}
	for _, proxy := range config.Proxies {
		if err := models.ValidateProxyAllowedPorts(allowed, proxy, server.ConfigContent); err != nil {
			return err
		}
	}
	return nil
}
