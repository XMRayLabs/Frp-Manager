package dao

import (
	"fmt"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type ClientQuery interface {
	ValidateClientSecret(clientID, clientSecret string) (*models.ClientEntity, error)
	AdminGetClientByClientID(clientID string) (*models.Client, error)
	GetClientByClientID(userInfo models.UserInfo, clientID string) (*models.Client, error)
	GetClientsByClientIDs(userInfo models.UserInfo, clientIDs []string) ([]*models.Client, error)
	GetClientByFilter(userInfo models.UserInfo, client *models.ClientEntity, shadow *bool) (*models.ClientEntity, error)
	GetClientByOriginClientID(originClientID string) (*models.ClientEntity, error)
	ListClients(userInfo models.UserInfo, page, pageSize int) ([]*models.ClientEntity, error)
	ListClientsWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ClientEntity, error)
	GetAllClients(userInfo models.UserInfo) ([]*models.ClientEntity, error)
	CountClients(userInfo models.UserInfo) (int64, error)
	CountClientsWithKeyword(userInfo models.UserInfo, keyword string) (int64, error)
	CountConfiguredClients(userInfo models.UserInfo) (int64, error)
	CountClientsInShadow(userInfo models.UserInfo, clientID string) (int64, error)
	GetClientIDsInShadowByClientID(userInfo models.UserInfo, clientID string) ([]string, error)
	GetClientIDsInShadowByClientIDs(userInfo models.UserInfo, clientIDs []string) (map[string][]string, error)
	AdminGetClientIDsInShadowByClientID(clientID string) ([]string, error)
}
type ClientMutation interface {
	CreateClient(userInfo models.UserInfo, client *models.ClientEntity) error
	DeleteClient(userInfo models.UserInfo, clientID string) error
	UpdateClient(userInfo models.UserInfo, client *models.ClientEntity) error
	AdminUpdateClientLastSeen(clientID string) error
}

type clientQuery struct{ *queryImpl }
type clientMutation struct{ *mutationImpl }

func newClientQuery(base *queryImpl) ClientQuery          { return &clientQuery{base} }
func newClientMutation(base *mutationImpl) ClientMutation { return &clientMutation{base} }

func (q *clientQuery) ValidateClientSecret(clientID, clientSecret string) (*models.ClientEntity, error) {
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("invalid client id or client secret")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Client{}
	err := db.
		Where(&models.Client{ClientEntity: &models.ClientEntity{
			ClientID: clientID,
		}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	if !utils.SecureStringEqual(c.ConnectSecret, clientSecret) {
		return nil, fmt.Errorf("invalid client secret")
	}
	return c.ClientEntity, nil
}

func (q *clientQuery) AdminGetClientByClientID(clientID string) (*models.Client, error) {
	if clientID == "" {
		return nil, fmt.Errorf("invalid client id")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Client{}
	err := db.
		Where(&models.Client{ClientEntity: &models.ClientEntity{
			ClientID: clientID,
		}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (q *clientQuery) GetClientByClientID(userInfo models.UserInfo, clientID string) (*models.Client, error) {
	if clientID == "" {
		return nil, fmt.Errorf("invalid client id")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Client{}
	err := scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: clientID}}).
		First(c).Error
	if err != nil {
		if err := db.
			Where("tenant_id = ?", userInfo.GetTenantID()).
			Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: clientID}}).
			First(c).Error; err != nil {
			return nil, err
		}
		if len(c.OriginClientID) == 0 {
			return nil, err
		}
		if err := CanAccessClient(q.ctx, userInfo, c.OriginClientID, defs.RBACActionView); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (q *clientQuery) GetClientsByClientIDs(userInfo models.UserInfo, clientIDs []string) ([]*models.Client, error) {
	if len(clientIDs) == 0 {
		return nil, fmt.Errorf("invalid client ids")
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	cs := []*models.Client{}
	err := scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where("client_id IN ?", clientIDs).Find(&cs).Error
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (q *clientQuery) GetClientByFilter(userInfo models.UserInfo, client *models.ClientEntity, shadow *bool) (*models.ClientEntity, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	filter := &models.ClientEntity{}
	if len(client.ClientID) != 0 {
		filter.ClientID = client.ClientID
	}
	if len(client.OriginClientID) != 0 {
		filter.OriginClientID = client.OriginClientID
	}
	if len(client.ConnectSecret) != 0 {
		filter.ConnectSecret = client.ConnectSecret
	}
	if len(client.ServerID) != 0 {
		filter.ServerID = client.ServerID
	}
	if shadow != nil {
		filter.IsShadow = *shadow
	}
	c := &models.Client{}

	var err error
	if len(filter.OriginClientID) != 0 {
		err = db.Where("tenant_id = ? AND user_id = ?", userInfo.GetTenantID(), userInfo.GetUserID()).
			Where(filter).
			First(c).Error
		if err == nil {
			return c.ClientEntity, nil
		}

		origin, err := q.GetClientByClientID(userInfo, filter.OriginClientID)
		if err != nil {
			return nil, err
		}
		err = db.Where("tenant_id = ?", origin.TenantID).Where(filter).First(c).Error
	} else {
		err = scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
			Where(filter).
			First(c).Error
	}
	if err != nil {
		return nil, err
	}
	return c.ClientEntity, nil
}

func (q *clientQuery) GetClientByOriginClientID(originClientID string) (*models.ClientEntity, error) {
	if originClientID == "" {
		return nil, fmt.Errorf("invalid origin client id")
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	c := &models.Client{}
	err := db.
		Where(&models.Client{ClientEntity: &models.ClientEntity{
			OriginClientID: originClientID,
		}}).
		First(c).Error
	if err != nil {
		return nil, err
	}
	return c.ClientEntity, nil
}

func (m *clientMutation) CreateClient(userInfo models.UserInfo, client *models.ClientEntity) error {
	client.UserID = userInfo.GetUserID()
	client.TenantID = userInfo.GetTenantID()
	c := &models.Client{
		ClientEntity: client,
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	if err := db.Create(c).Error; err != nil {
		return err
	}
	grantOwnerPermissions(m.ctx, userInfo, defs.RBACObjClient, client.ClientID)
	return nil
}

func (m *clientMutation) DeleteClient(userInfo models.UserInfo, clientID string) error {
	if clientID == "" {
		return fmt.Errorf("invalid client id")
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	client := &models.Client{}
	if err := db.Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: clientID}}).First(client).Error; err != nil {
		return err
	}
	if err := canAccessResource(m.ctx, userInfo, defs.RBACObjClient, clientID, ownedResource{
		tenantID: client.TenantID,
		userID:   client.UserID,
	}, defs.RBACActionEdit); err != nil {
		return err
	}
	if err := db.Unscoped().Where(&models.Client{
		ClientEntity: &models.ClientEntity{
			ClientID: clientID,
		},
	}).Or(&models.Client{
		ClientEntity: &models.ClientEntity{
			OriginClientID: clientID,
		},
	}).Delete(&models.Client{}).Error; err != nil {
		return err
	}
	revokeResourcePermissions(m.ctx, defs.RBACObjClient, clientID, userInfo.GetTenantID())
	return nil
}

func (m *clientMutation) UpdateClient(userInfo models.UserInfo, client *models.ClientEntity) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	old := &models.Client{}
	if err := db.Where(&models.Client{ClientEntity: &models.ClientEntity{ClientID: client.ClientID}}).First(old).Error; err != nil {
		return err
	}
	err := canAccessResource(m.ctx, userInfo, defs.RBACObjClient, client.ClientID, ownedResource{
		tenantID: old.TenantID,
		userID:   old.UserID,
	}, defs.RBACActionEdit)
	if err != nil && len(old.OriginClientID) != 0 {
		err = canAccessResource(m.ctx, userInfo, defs.RBACObjClient, old.OriginClientID, ownedResource{
			tenantID: old.TenantID,
			userID:   old.UserID,
		}, defs.RBACActionEdit)
	}
	if err != nil {
		return err
	}
	client.UserID = old.UserID
	client.TenantID = old.TenantID
	return db.Save(&models.Client{ClientEntity: client}).Error
}

func (q *clientQuery) ListClients(userInfo models.UserInfo, page, pageSize int) ([]*models.ClientEntity, error) {
	if page < 1 || pageSize < 1 {
		return nil, fmt.Errorf("invalid page or page size")
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	offset := (page - 1) * pageSize

	var clients []*models.Client
	err := scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where(
			db.Where(
				normalClientFilter(db),
			),
		).Order("client_id ASC").Offset(offset).Limit(pageSize).Find(&clients).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(clients, func(c *models.Client, _ int) *models.ClientEntity {
		return c.ClientEntity
	}), nil
}

func (q *clientQuery) ListClientsWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ClientEntity, error) {
	// 閸欘亣骞忛崣鏍ㄧ梾shadow娑撴攦onfig閺堝绗㈢憲?
	// 閹存潒sShadow閻ㄥ垻lient
	if page < 1 || pageSize < 1 || len(keyword) == 0 {
		return nil, fmt.Errorf("invalid page or page size or keyword")
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	offset := (page - 1) * pageSize

	var clients []*models.Client
	err := scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where("client_id like ?", "%"+keyword+"%").
		Where(normalClientFilter(db)).
		Order("client_id ASC").
		Offset(offset).Limit(pageSize).Find(&clients).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(clients, func(c *models.Client, _ int) *models.ClientEntity {
		return c.ClientEntity
	}), nil
}

func (q *clientQuery) GetAllClients(userInfo models.UserInfo) ([]*models.ClientEntity, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var clients []*models.Client
	err := scopeOwnedOrShared(db, q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Find(&clients).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(clients, func(c *models.Client, _ int) *models.ClientEntity {
		return c.ClientEntity
	}), nil
}

func (q *clientQuery) CountClients(userInfo models.UserInfo) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeOwnedOrShared(db.Model(&models.Client{}), q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where(normalClientFilter(db)).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *clientQuery) CountClientsWithKeyword(userInfo models.UserInfo, keyword string) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeOwnedOrShared(db.Model(&models.Client{}), q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where(normalClientFilter(db)).Where("client_id like ?", "%"+keyword+"%").Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *clientQuery) CountConfiguredClients(userInfo models.UserInfo) (int64, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err := scopeOwnedOrShared(db.Model(&models.Client{}), q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).
		Where(normalClientFilter(db)).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *clientQuery) CountClientsInShadow(userInfo models.UserInfo, clientID string) (int64, error) {
	origin, err := q.GetClientByClientID(userInfo, clientID)
	if err != nil {
		return 0, err
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var count int64
	err = db.Model(&models.Client{}).
		Where(&models.Client{
			ClientEntity: &models.ClientEntity{
				TenantID:       origin.TenantID,
				OriginClientID: clientID,
			}}).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *clientQuery) GetClientIDsInShadowByClientID(userInfo models.UserInfo, clientID string) ([]string, error) {
	origin, err := q.GetClientByClientID(userInfo, clientID)
	if err != nil {
		return nil, err
	}

	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var clients []*models.Client
	err = db.Where(&models.Client{
		ClientEntity: &models.ClientEntity{
			TenantID:       origin.TenantID,
			OriginClientID: clientID,
		}}).Find(&clients).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(clients, func(c *models.Client, _ int) string {
		return c.ClientID
	}), nil
}

func (q *clientQuery) GetClientIDsInShadowByClientIDs(userInfo models.UserInfo, clientIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(clientIDs))
	if len(clientIDs) == 0 {
		return result, nil
	}

	accessible, err := q.GetClientsByClientIDs(userInfo, clientIDs)
	if err != nil {
		return nil, err
	}
	allowedIDs := lo.Map(accessible, func(client *models.Client, _ int) string {
		return client.ClientID
	})
	if len(allowedIDs) == 0 {
		return result, nil
	}

	var rows []struct {
		ClientID       string
		OriginClientID string
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	err = db.Model(&models.Client{}).
		Select("client_id", "origin_client_id").
		Where("origin_client_id IN ?", allowedIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.OriginClientID] = append(result[row.OriginClientID], row.ClientID)
	}
	return result, nil
}

func (q *clientQuery) AdminGetClientIDsInShadowByClientID(clientID string) ([]string, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	var clients []*models.Client
	err := db.Where(&models.Client{
		ClientEntity: &models.ClientEntity{
			OriginClientID: clientID,
		}}).Find(&clients).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(clients, func(c *models.Client, _ int) string {
		return c.ClientID
	}), nil
}

func (m *clientMutation) AdminUpdateClientLastSeen(clientID string) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Model(&models.Client{
		ClientEntity: &models.ClientEntity{
			ClientID: clientID,
		}}).Update("last_seen_at", time.Now()).Error
}

func normalClientFilter(db *gorm.DB) *gorm.DB {
	// 1. 濞岊摴hadow鏉╁洨娈戦懓涔ient
	// 2. shadow鏉╁洨娈憇hadow client
	// 3. 闂堢偘澶嶉弮鎯板Ν閻?
	return db.Where(
		db.Where("origin_client_id is NULL").
			Or("is_shadow = ?", true).
			Or("LENGTH(origin_client_id) = ?", 0),
	).
		Where(
			db.Where(
				db.Where("ephemeral is NULL").
					Or("ephemeral = ?", false),
			).Or(
				db.Where("ephemeral = ?", true).
					Where("last_seen_at > ?", time.Now().Add(-5*time.Minute)),
			),
		)
}
