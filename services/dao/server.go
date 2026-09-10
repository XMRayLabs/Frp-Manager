package dao

import (
	"fmt"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type ServerQuery interface {
	GetDefaultServer() (*models.ServerEntity, error)
	ValidateServerSecret(serverID string, secret string) (*models.ServerEntity, error)
	AdminGetServerByServerID(serverID string) (*models.ServerEntity, error)
	GetServerByServerID(userInfo models.UserInfo, serverID string) (*models.ServerEntity, error)
	GetServersByServerIDs(userInfo models.UserInfo, serverIDs []string) ([]*models.ServerEntity, error)
	ListServers(userInfo models.UserInfo, page, pageSize int) ([]*models.ServerEntity, error)
	ListServersWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ServerEntity, error)
	CountServers(userInfo models.UserInfo) (int64, error)
	CountServersWithKeyword(userInfo models.UserInfo, keyword string) (int64, error)
	CountConfiguredServers(userInfo models.UserInfo) (int64, error)
}

type ServerMutation interface {
	InitDefaultServer(serverIP string)
	UpdateDefaultServer(c *models.Server) error
	CreateServer(userInfo models.UserInfo, server *models.ServerEntity) error
	DeleteServer(userInfo models.UserInfo, serverID string) error
	UpdateServer(userInfo models.UserInfo, server *models.ServerEntity) error
}

type serverQuery struct{ *queryImpl }
type serverMutation struct{ *mutationImpl }

func newServerQuery(base *queryImpl) ServerQuery          { return &serverQuery{base} }
func newServerMutation(base *mutationImpl) ServerMutation { return &serverMutation{base} }

func (m *serverMutation) InitDefaultServer(serverIP string) {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	db.Where(&models.Server{
		ServerEntity: &models.ServerEntity{
			ServerID: defs.DefaultServerID,
		},
	}).Attrs(&models.Server{
		ServerEntity: &models.ServerEntity{
			ServerID:      defs.DefaultServerID,
			ServerIP:      serverIP,
			ConnectSecret: uuid.New().String(),
		},
	}).FirstOrCreate(&models.Server{})
}

func (q *serverQuery) GetDefaultServer() (*models.ServerEntity, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Server{}
	err := db.
		Where(&models.Server{ServerEntity: &models.ServerEntity{
			ServerID: defs.DefaultServerID,
		}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c.ServerEntity, nil
}

func (m *serverMutation) UpdateDefaultServer(c *models.Server) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	c.ServerID = defs.DefaultServerID
	var previous models.Server
	if err := db.Where("server_id = ?", defs.DefaultServerID).First(&previous).Error; err != nil {
		return err
	}
	c.DeviceID, c.RuntimeID = previous.DeviceID, previous.RuntimeID
	err := db.Where(&models.Server{
		ServerEntity: &models.ServerEntity{
			ServerID: defs.DefaultServerID,
		}}).Save(c).Error
	if err != nil {
		return err
	}
	return nil
}

func (q *serverQuery) ValidateServerSecret(serverID, secret string) (*models.ServerEntity, error) {
	if secret == "" {
		return nil, fmt.Errorf("device credentials expired or invalid; enroll again explicitly")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var rows []*models.Server
	query := db.Where("connect_secret = ?", secret)

	if err := query.Limit(2).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != 1 || !utils.SecureStringEqual(rows[0].ConnectSecret, secret) {
		return nil, fmt.Errorf("device credentials expired or invalid; enroll again explicitly")
	}

	return rows[0].ServerEntity, nil
}

func (q *serverQuery) AdminGetServerByServerID(serverID string) (*models.ServerEntity, error) {
	if serverID == "" {
		return nil, fmt.Errorf("invalid server id")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Server{}
	err := db.
		Where(&models.Server{ServerEntity: &models.ServerEntity{
			ServerID: serverID,
		}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c.ServerEntity, nil
}

func (q *serverQuery) GetServerByServerID(userInfo models.UserInfo, serverID string) (*models.ServerEntity, error) {
	if serverID == "" {
		return nil, fmt.Errorf("invalid server id")
	}
	if userInfo.IsAdmin() && serverID == defs.DefaultServerID {
		return q.GetDefaultServer()
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Server{}
	err := scopeUsableServers(db, userInfo).
		Where(&models.Server{ServerEntity: &models.ServerEntity{ServerID: serverID}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c.ServerEntity, nil
}

func (q *serverQuery) GetServersByServerIDs(userInfo models.UserInfo, serverIDs []string) ([]*models.ServerEntity, error) {
	if len(serverIDs) == 0 {
		return nil, fmt.Errorf("invalid server ids")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var servers []*models.Server
	if err := scopeUsableServers(db, userInfo).Where("server_id IN ?", serverIDs).Find(&servers).Error; err != nil {
		return nil, err
	}
	return lo.Map(servers, func(server *models.Server, _ int) *models.ServerEntity {
		return server.ServerEntity
	}), nil
}

func (m *serverMutation) CreateServer(userInfo models.UserInfo, server *models.ServerEntity) error {
	if !userInfo.IsAdmin() {
		return fmt.Errorf("仅网站管理员可创建服务端")
	}
	server.UserID = userInfo.GetUserID()
	server.TenantID = userInfo.GetTenantID()
	c := &models.Server{
		ServerEntity: server,
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	if err := db.Create(c).Error; err != nil {
		return err
	}
	grantOwnerPermissions(m.ctx, userInfo, defs.RBACObjServer, server.ServerID)
	return nil
}

func (m *serverMutation) DeleteServer(userInfo models.UserInfo, serverID string) error {
	models.NodeIdentityMu.Lock()
	defer models.NodeIdentityMu.Unlock()
	if serverID == "" {
		return fmt.Errorf("invalid server id")
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	server := &models.Server{}
	if err := db.Where(&models.Server{ServerEntity: &models.ServerEntity{ServerID: serverID}}).First(server).Error; err != nil {
		return err
	}
	if err := canAccessResource(m.ctx, userInfo, defs.RBACObjServer, serverID, ownedResource{
		tenantID: server.TenantID,
		userID:   server.UserID,
	}, defs.RBACActionEdit); err != nil {
		return err
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("server_id = ?", serverID).Delete(&models.ProxyConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("server_id = ? AND origin_client_id IS NOT NULL AND origin_client_id <> ?", serverID, "").Delete(&models.Client{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Client{}).Where("server_id = ?", serverID).Updates(map[string]interface{}{"server_id": "", "config_content": nil}).Error; err != nil {
			return err
		}
		if tx.Migrator().HasTable(&models.LanguageGroupServer{}) {
			if err := tx.Where("server_id = ?", serverID).Delete(&models.LanguageGroupServer{}).Error; err != nil {
				return err
			}
		}
		return tx.Unscoped().Where("server_id = ?", serverID).Delete(&models.Server{}).Error
	}); err != nil {
		return err
	}

	if manager := m.ctx.GetApp().GetClientsManager(); manager != nil {
		manager.Remove(serverID)
	}
	revokeResourcePermissions(m.ctx, defs.RBACObjServer, serverID, userInfo.GetTenantID())
	return nil
}

func (m *serverMutation) UpdateServer(userInfo models.UserInfo, server *models.ServerEntity) error {
	if userInfo.IsAdmin() && server.ServerID == defs.DefaultServerID {
		return m.UpdateDefaultServer(&models.Server{ServerEntity: server})
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	old := &models.Server{}
	if err := db.Where(&models.Server{ServerEntity: &models.ServerEntity{ServerID: server.ServerID}}).First(old).Error; err != nil {
		return err
	}
	if err := canAccessResource(m.ctx, userInfo, defs.RBACObjServer, server.ServerID, ownedResource{
		tenantID: old.TenantID,
		userID:   old.UserID,
	}, defs.RBACActionEdit); err != nil {
		return err
	}
	server.DeviceID = old.DeviceID
	server.RuntimeID = old.RuntimeID
	server.UserID = old.UserID
	server.TenantID = old.TenantID
	return db.Save(&models.Server{ServerEntity: server}).Error
}

func (q *serverQuery) ListServers(userInfo models.UserInfo, page, pageSize int) ([]*models.ServerEntity, error) {
	if page < 1 || pageSize < 1 {
		return nil, fmt.Errorf("invalid page or page size")
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	offset := (page - 1) * pageSize

	var servers []*models.Server
	err := scopeUsableServers(db, userInfo).Order("server_id ASC").Offset(offset).Limit(pageSize).Find(&servers).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(servers, func(c *models.Server, _ int) *models.ServerEntity {
		return c.ServerEntity
	}), nil
}

func (q *serverQuery) ListServersWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ServerEntity, error) {
	if page < 1 || pageSize < 1 || len(keyword) == 0 {
		return nil, fmt.Errorf("invalid page or page size or keyword")
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	offset := (page - 1) * pageSize

	var servers []*models.Server
	err := scopeUsableServers(db, userInfo).
		Where("server_id like ?", "%"+keyword+"%").
		Order("server_id ASC").
		Offset(offset).Limit(pageSize).Find(&servers).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(servers, func(c *models.Server, _ int) *models.ServerEntity {
		return c.ServerEntity
	}), nil
}

func (q *serverQuery) CountServers(userInfo models.UserInfo) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeUsableServers(db.Model(&models.Server{}), userInfo).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *serverQuery) CountServersWithKeyword(userInfo models.UserInfo, keyword string) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeUsableServers(db.Model(&models.Server{}), userInfo).
		Where("server_id like ?", "%"+keyword+"%").Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *serverQuery) CountConfiguredServers(userInfo models.UserInfo) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeUsableServers(db.Model(&models.Server{}), userInfo).Not(
		&models.Server{
			ServerEntity: &models.ServerEntity{
				ConfigContent: []byte{},
			},
		},
	).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}
