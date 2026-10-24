package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/fatedier/frp/pkg/naming"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProxyQuery interface {
	GetProxyStatsByClientID(userInfo models.UserInfo, clientID string) ([]*models.ProxyStatsEntity, error)
	GetProxyStatsByServerID(userInfo models.UserInfo, serverID string) ([]*models.ProxyStatsEntity, error)
	GetAllProxyStats(userInfo models.UserInfo) ([]*models.ProxyStatsEntity, error)
	AdminGetTenantProxyStats(tenantID int) ([]*models.ProxyStatsEntity, error)
	AdminGetAllProxyStats(tx *gorm.DB) ([]*models.ProxyStatsEntity, error)
	AdminGetProxyConfigByClientIDAndName(clientID string, name string) (*models.ProxyConfig, error)
	GetProxyConfigsByClientID(userInfo models.UserInfo, clientID string) ([]*models.ProxyConfigEntity, error)
	GetProxyConfigByFilter(userInfo models.UserInfo, proxyConfig *models.ProxyConfigEntity) (*models.ProxyConfig, error)
	ListProxyConfigsWithFilters(userInfo models.UserInfo, page, pageSize int, filters *models.ProxyConfigEntity) ([]*models.ProxyConfig, error)
	AdminListProxyConfigsWithFilters(filters *models.ProxyConfigEntity) ([]*models.ProxyConfig, error)
	ListProxyConfigsWithFiltersAndKeyword(userInfo models.UserInfo, page, pageSize int, filters *models.ProxyConfigEntity, keyword string) ([]*models.ProxyConfig, error)
	ListProxyConfigsWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ProxyConfig, error)
	ListProxyConfigs(userInfo models.UserInfo, page, pageSize int) ([]*models.ProxyConfig, error)
	ListProxyConfigsByIDs(userInfo models.UserInfo, proxyIDs []uint32) ([]*models.ProxyConfig, error)
	GetProxyConfigByOriginClientIDAndName(userInfo models.UserInfo, clientID string, name string) (*models.ProxyConfig, error)
	CountProxyConfigs(userInfo models.UserInfo) (int64, error)
	CountProxyConfigsWithFilters(userInfo models.UserInfo, filters *models.ProxyConfigEntity) (int64, error)
	CountProxyConfigsWithFiltersAndKeyword(userInfo models.UserInfo, filters *models.ProxyConfigEntity, keyword string) (int64, error)
	GetProxyConfigsByWorkerId(userInfo models.UserInfo, workerID string) ([]*models.ProxyConfig, error)
}

type ProxyMutation interface {
	AdminUpdateProxyStats(srv *models.ServerEntity, inputs []*pb.ProxyInfo) error
	AdminCreateProxyConfig(proxyCfg *models.ProxyConfig) error
	RebuildProxyConfigFromClient(userInfo models.UserInfo, client *models.Client) error
	CreateProxyConfig(userInfo models.UserInfo, proxyCfg *models.ProxyConfigEntity) error
	UpdateProxyConfig(userInfo models.UserInfo, proxyCfg *models.ProxyConfig) error
	DeleteProxyConfig(userInfo models.UserInfo, clientID, name string) error
	DeleteProxyConfigsByClientIDOrOriginClientID(userInfo models.UserInfo, clientID string) error
	DeleteProxyConfigsByClientID(userInfo models.UserInfo, clientID string) error
}

type proxyQuery struct{ *queryImpl }
type proxyMutation struct{ *mutationImpl }

func newProxyQuery(base *queryImpl) ProxyQuery          { return &proxyQuery{base} }
func newProxyMutation(base *mutationImpl) ProxyMutation { return &proxyMutation{base} }

func (q *proxyQuery) GetProxyStatsByClientID(userInfo models.UserInfo, clientID string) ([]*models.ProxyStatsEntity, error) {
	if clientID == "" {
		return nil, fmt.Errorf("invalid client id")
	}
	if err := CanAccessClient(q.ctx, userInfo, clientID, defs.RBACActionView); err != nil {
		return nil, err
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	list := []*models.ProxyStats{}
	err := db.Where("tenant_id = ? AND (client_id = ? OR origin_client_id = ?)", userInfo.GetTenantID(), clientID, clientID).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyStats, _ int) *models.ProxyStatsEntity {
		return item.ProxyStatsEntity
	}), nil
}

func (q *proxyQuery) GetProxyStatsByServerID(userInfo models.UserInfo, serverID string) ([]*models.ProxyStatsEntity, error) {
	if serverID == "" {
		return nil, fmt.Errorf("invalid server id")
	}
	if err := CanAccessServer(q.ctx, userInfo, serverID, defs.RBACActionView); err != nil {
		return nil, err
	}
	rows, err := q.GetAllProxyStats(userInfo)
	if err != nil {
		return nil, err
	}
	return lo.Filter(rows, func(row *models.ProxyStatsEntity, _ int) bool { return row.ServerID == serverID }), nil
}

func (m *proxyMutation) AdminUpdateProxyStats(srv *models.ServerEntity, inputs []*pb.ProxyInfo) error {
	if srv == nil || srv.ServerID == "" {
		return fmt.Errorf("invalid server id")
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Transaction(func(tx *gorm.DB) error {
		// Shared servers host clients belonging to many users. Match complete
		// wire names from each client's config instead of stripping the server
		// owner's username or splitting names that legitimately contain dots.
		var clients []*models.Client
		query := tx.Where("server_id = ?", srv.ServerID)
		if srv.ServerID != defs.DefaultServerID {
			query = query.Where("tenant_id = ?", srv.TenantID)
		}
		if err := query.Find(&clients).Error; err != nil {
			return err
		}
		type target struct {
			client          *models.Client
			name, proxyType string
		}
		targets := map[string][]target{}
		for _, client := range clients {
			cfg, err := client.GetConfigContent()
			if err != nil || cfg == nil {
				continue
			}
			for _, proxy := range cfg.Proxies {
				base := proxy.GetBaseConfig()
				wireName := naming.AddUserPrefix(cfg.User, base.Name)
				targets[wireName] = append(targets[wireName], target{client, base.Name, base.Type})
			}
		}
		type identity struct {
			tenant, owner  int
			clientID, name string
		}
		var oldRows []*models.ProxyStats
		query = tx.Where("server_id = ?", srv.ServerID)
		if srv.ServerID != defs.DefaultServerID {
			query = query.Where("tenant_id = ?", srv.TenantID)
		}
		if err := query.Find(&oldRows).Error; err != nil {
			return err
		}
		oldByIdentity := map[identity]*models.ProxyStats{}
		for _, old := range oldRows {
			oldByIdentity[identity{old.TenantID, old.UserID, old.ClientID, old.Name}] = old
		}
		results := map[identity]*models.ProxyStats{}
		now := time.Now()
		for _, input := range inputs {
			if input == nil {
				continue
			}
			matches := targets[input.GetName()]
			if len(matches) != 1 || matches[0].proxyType != input.GetType() {
				continue
			}
			match := matches[0]
			key := identity{match.client.TenantID, match.client.UserID, match.client.ClientID, match.name}
			row := &models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
				ServerID: srv.ServerID, ClientID: match.client.ClientID, OriginClientID: match.client.OriginClientID,
				Name: match.name, Type: match.proxyType, UserID: match.client.UserID, TenantID: match.client.TenantID,
				TodayTrafficIn: input.GetTodayTrafficIn(), TodayTrafficOut: input.GetTodayTrafficOut(),
			}}
			if old := oldByIdentity[key]; old != nil {
				row.ProxyID, row.CreatedAt = old.ProxyID, old.CreatedAt
				row.HistoryTrafficIn, row.HistoryTrafficOut = old.HistoryTrafficIn, old.HistoryTrafficOut
				if !utils.IsSameDay(now, old.UpdatedAt) || input.GetFirstSync() {
					row.HistoryTrafficIn += old.TodayTrafficIn
					row.HistoryTrafficOut += old.TodayTrafficOut
				}
			}
			results[key] = row
		}
		if len(results) > 0 {
			return tx.Save(lo.Values(results)).Error
		}
		return nil
	})
}

func (q *proxyQuery) AdminGetTenantProxyStats(tenantID int) ([]*models.ProxyStatsEntity, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	list := []*models.ProxyStats{}
	err := db.
		Where(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			TenantID: tenantID,
		}}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyStats, _ int) *models.ProxyStatsEntity {
		return item.ProxyStatsEntity
	}), nil
}

func (q *proxyQuery) AdminGetAllProxyStats(tx *gorm.DB) ([]*models.ProxyStatsEntity, error) {
	db := tx
	list := []*models.ProxyStats{}
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyStats, _ int) *models.ProxyStatsEntity {
		return item.ProxyStatsEntity
	}), nil
}

func (m *proxyMutation) AdminCreateProxyConfig(proxyCfg *models.ProxyConfig) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Create(proxyCfg).Error
}

// RebuildProxyConfigFromClient rebuild proxy from client
// skip stopped proxy
func (m *proxyMutation) RebuildProxyConfigFromClient(userInfo models.UserInfo, client *models.Client) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	query := NewQuery(m.ctx)

	pxyCfgs, err := utils.LoadProxiesFromContent(client.ConfigContent)
	if err != nil {
		return err
	}

	proxyConfigEntities := []*models.ProxyConfig{}

	for _, pxyCfg := range pxyCfgs {
		proxyCfg := &models.ProxyConfig{
			ProxyConfigEntity: &models.ProxyConfigEntity{},
		}
		if oldProxyCfg, err := query.GetProxyConfigByOriginClientIDAndName(userInfo, client.ClientID, pxyCfg.GetBaseConfig().Name); err == nil {
			logger.Logger(context.Background()).WithError(err).Warnf("proxy config already exist, will be override, clientID: [%s], name: [%s]",
				client.ClientID, pxyCfg.GetBaseConfig().Name)
			proxyCfg.Model = oldProxyCfg.Model
		}

		if err := proxyCfg.FillClientConfig(client.ClientEntity); err != nil {
			return err
		}

		if err := proxyCfg.FillTypedProxyConfig(pxyCfg); err != nil {
			return err
		}

		proxyConfigEntities = append(proxyConfigEntities, proxyCfg)
	}

	if err := m.DeleteProxyConfigsByClientIDOrOriginClientID(userInfo, client.ClientID); err != nil {
		return err
	}

	if len(proxyConfigEntities) == 0 {
		return nil
	}

	return db.Save(proxyConfigEntities).Error
}

func (q *proxyQuery) AdminGetProxyConfigByClientIDAndName(clientID string, name string) (*models.ProxyConfig, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	proxyCfg := &models.ProxyConfig{}
	err := db.
		Where(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{
			ClientID: clientID,
			Name:     name,
		}}).
		First(proxyCfg).Error
	if err != nil {
		return nil, err
	}
	return proxyCfg, nil
}

func (q *proxyQuery) GetProxyConfigsByClientID(userInfo models.UserInfo, clientID string) ([]*models.ProxyConfigEntity, error) {
	if clientID == "" {
		return nil, fmt.Errorf("invalid client id")
	}
	if err := CanAccessClient(q.ctx, userInfo, clientID, defs.RBACActionView); err != nil {
		return nil, err
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	list := []*models.ProxyConfig{}
	err := db.
		Where("tenant_id = ?", userInfo.GetTenantID()).
		Where(db.Where("client_id = ?", clientID).Or("origin_client_id = ?", clientID)).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyConfig, _ int) *models.ProxyConfigEntity {
		return item.ProxyConfigEntity
	}), nil
}

func (q *proxyQuery) GetProxyConfigByFilter(userInfo models.UserInfo, proxyConfig *models.ProxyConfigEntity) (*models.ProxyConfig, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	filter := &models.ProxyConfigEntity{}

	if len(proxyConfig.ClientID) != 0 {
		filter.ClientID = proxyConfig.ClientID
	}
	if len(proxyConfig.OriginClientID) != 0 {
		filter.OriginClientID = proxyConfig.OriginClientID
	}
	if len(proxyConfig.Name) != 0 {
		filter.Name = proxyConfig.Name
	}
	if len(proxyConfig.Type) != 0 {
		filter.Type = proxyConfig.Type
	}
	if len(proxyConfig.ServerID) != 0 {
		filter.ServerID = proxyConfig.ServerID
	}

	filter.TenantID = userInfo.GetTenantID()

	respProxyCfg := &models.ProxyConfig{}
	err := db.
		Where(&models.ProxyConfig{ProxyConfigEntity: filter}).
		First(respProxyCfg).Error
	if err != nil {
		return nil, err
	}
	accessClientID := respProxyCfg.ClientID
	if len(respProxyCfg.OriginClientID) != 0 {
		accessClientID = respProxyCfg.OriginClientID
	}
	if err := CanAccessClient(q.ctx, userInfo, accessClientID, defs.RBACActionView); err != nil {
		return nil, err
	}
	return respProxyCfg, nil
}

func (q *proxyQuery) GetAllProxyStats(userInfo models.UserInfo) ([]*models.ProxyStatsEntity, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	query := db.Model(&models.ProxyStats{}).Where("tenant_id = ?", userInfo.GetTenantID())

	if !userInfo.IsAdmin() {
		nodes := organizationScope(db.Model(&models.Client{}), q.ctx, userInfo, defs.RBACObjClient, "client_id", defs.RBACActionView).Select("client_id")
		query = query.Where("client_id IN (?) OR origin_client_id IN (?)", nodes, nodes)
	}

	var list []*models.ProxyStats
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyStats, _ int) *models.ProxyStatsEntity {
		return item.ProxyStatsEntity
	}), nil
}

func cloneProxyConfigFilters(filters *models.ProxyConfigEntity) *models.ProxyConfigEntity {
	if filters == nil {
		return &models.ProxyConfigEntity{}
	}
	clone := *filters
	clone.UserID = 0
	clone.TenantID = 0
	return &clone
}

func (q *proxyQuery) proxyConfigQuery(userInfo models.UserInfo, filters *models.ProxyConfigEntity, action defs.RBACAction) (*gorm.DB, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	query := db.Model(&models.ProxyConfig{}).Where("tenant_id = ?", userInfo.GetTenantID())
	filter := cloneProxyConfigFilters(filters)
	clientID := filter.OriginClientID
	filter.OriginClientID = ""

	if len(clientID) != 0 {
		if err := CanAccessClient(q.ctx, userInfo, clientID, action); err != nil {
			return nil, err
		}
		query = query.Where(db.Where("origin_client_id = ?", clientID).Or("client_id = ?", clientID))
	} else {
		query = organizationScope(query, q.ctx, userInfo, defs.RBACObjClient, "client_id", action)
	}

	return query.Where(&models.ProxyConfig{ProxyConfigEntity: filter}), nil
}

func (q *proxyQuery) ListProxyConfigsWithFilters(userInfo models.UserInfo, page, pageSize int, filters *models.ProxyConfigEntity) ([]*models.ProxyConfig, error) {
	if page < 1 || pageSize < 1 {
		return nil, fmt.Errorf("invalid page or page size")
	}

	offset := (page - 1) * pageSize

	var proxyConfigs []*models.ProxyConfig
	query, err := q.proxyConfigQuery(userInfo, filters, defs.RBACActionView)
	if err != nil {
		return nil, err
	}
	err = query.Offset(offset).Limit(pageSize).Find(&proxyConfigs).Error
	if err != nil {
		return nil, err
	}

	return proxyConfigs, nil
}

func (q *proxyQuery) AdminListProxyConfigsWithFilters(filters *models.ProxyConfigEntity) ([]*models.ProxyConfig, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()

	var proxyConfigs []*models.ProxyConfig
	err := db.Where(&models.ProxyConfig{
		ProxyConfigEntity: filters,
	}).Where(filters).Find(&proxyConfigs).Error
	if err != nil {
		return nil, err
	}

	return proxyConfigs, nil
}

func (q *proxyQuery) ListProxyConfigsWithFiltersAndKeyword(userInfo models.UserInfo, page, pageSize int, filters *models.ProxyConfigEntity, keyword string) ([]*models.ProxyConfig, error) {
	if page < 1 || pageSize < 1 || len(keyword) == 0 {
		return nil, fmt.Errorf("invalid page or page size or keyword")
	}

	offset := (page - 1) * pageSize

	var proxyConfigs []*models.ProxyConfig
	query, err := q.proxyConfigQuery(userInfo, filters, defs.RBACActionView)
	if err != nil {
		return nil, err
	}
	err = query.Where("name like ?", "%"+keyword+"%").Offset(offset).Limit(pageSize).Find(&proxyConfigs).Error
	if err != nil {
		return nil, err
	}

	return proxyConfigs, nil
}

func (q *proxyQuery) ListProxyConfigsWithKeyword(userInfo models.UserInfo, page, pageSize int, keyword string) ([]*models.ProxyConfig, error) {
	return q.ListProxyConfigsWithFiltersAndKeyword(userInfo, page, pageSize, &models.ProxyConfigEntity{}, keyword)
}

func (q *proxyQuery) ListProxyConfigs(userInfo models.UserInfo, page, pageSize int) ([]*models.ProxyConfig, error) {
	return q.ListProxyConfigsWithFilters(userInfo, page, pageSize, &models.ProxyConfigEntity{})
}

func (q *proxyQuery) ListProxyConfigsByIDs(userInfo models.UserInfo, proxyIDs []uint32) ([]*models.ProxyConfig, error) {
	if len(proxyIDs) == 0 {
		return []*models.ProxyConfig{}, nil
	}

	const chunkSize = 500
	results := make([]*models.ProxyConfig, 0, len(proxyIDs))
	for start := 0; start < len(proxyIDs); start += chunkSize {
		end := min(start+chunkSize, len(proxyIDs))
		query, err := q.proxyConfigQuery(userInfo, &models.ProxyConfigEntity{}, defs.RBACActionView)
		if err != nil {
			return nil, err
		}
		var chunk []*models.ProxyConfig
		if err := query.Where("id IN ?", proxyIDs[start:end]).Find(&chunk).Error; err != nil {
			return nil, err
		}
		results = append(results, chunk...)
	}
	return results, nil
}

func (m *proxyMutation) CreateProxyConfig(userInfo models.UserInfo, proxyCfg *models.ProxyConfigEntity) error {
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	proxyCfg.UserID = userInfo.GetUserID()
	proxyCfg.TenantID = userInfo.GetTenantID()
	return db.Create(&models.ProxyConfig{ProxyConfigEntity: proxyCfg}).Error
}

func (m *proxyMutation) UpdateProxyConfig(userInfo models.UserInfo, proxyCfg *models.ProxyConfig) error {
	if proxyCfg.ID == 0 {
		return fmt.Errorf("invalid proxy config id")
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	old := &models.ProxyConfig{}
	if err := db.Where("tenant_id = ?", userInfo.GetTenantID()).Where(&models.ProxyConfig{Model: &gorm.Model{ID: proxyCfg.ID}}).First(old).Error; err != nil {
		return err
	}
	accessClientID := old.ClientID
	if len(old.OriginClientID) != 0 {
		accessClientID = old.OriginClientID
	}
	if err := CanAccessClient(m.ctx, userInfo, accessClientID, defs.RBACActionEdit); err != nil {
		return err
	}
	proxyCfg.UserID = old.UserID
	proxyCfg.TenantID = old.TenantID
	return db.Save(proxyCfg).Error
}

func (m *proxyMutation) DeleteProxyConfig(userInfo models.UserInfo, clientID, name string) error {
	if clientID == "" || name == "" {
		return fmt.Errorf("invalid client id or name")
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	proxyCfg := &models.ProxyConfig{}
	if err := db.
		Where("tenant_id = ? AND name = ?", userInfo.GetTenantID(), name).
		Where(db.Where("client_id = ?", clientID).Or("origin_client_id = ?", clientID)).
		First(proxyCfg).Error; err != nil {
		return err
	}
	accessClientID := proxyCfg.ClientID
	if len(proxyCfg.OriginClientID) != 0 {
		accessClientID = proxyCfg.OriginClientID
	}
	if err := CanAccessClient(m.ctx, userInfo, accessClientID, defs.RBACActionEdit); err != nil {
		return err
	}
	return db.Unscoped().Delete(proxyCfg).Error
}

func (m *proxyMutation) DeleteProxyConfigsByClientIDOrOriginClientID(userInfo models.UserInfo, clientID string) error {
	if clientID == "" {
		return fmt.Errorf("invalid client id")
	}
	if err := CanAccessClient(m.ctx, userInfo, clientID, defs.RBACActionEdit); err != nil {
		return err
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Unscoped().
		Where(
			db.Where(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{
				TenantID: userInfo.GetTenantID(),
				ClientID: clientID,
			}}).
				Or(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{
					TenantID:       userInfo.GetTenantID(),
					OriginClientID: clientID,
				}})).
		Where(db.Where("stopped is NULL").
			Or("stopped = ?", false)).
		Delete(&models.ProxyConfig{}).Error
}

func (m *proxyMutation) DeleteProxyConfigsByClientID(userInfo models.UserInfo, clientID string) error {
	if clientID == "" {
		return fmt.Errorf("invalid client id")
	}
	if err := CanAccessClient(m.ctx, userInfo, clientID, defs.RBACActionEdit); err != nil {
		return err
	}
	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Unscoped().
		Where(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{
			TenantID: userInfo.GetTenantID(),
			ClientID: clientID,
		}}).
		Delete(&models.ProxyConfig{}).Error
}

func (q *proxyQuery) GetProxyConfigByOriginClientIDAndName(userInfo models.UserInfo, clientID string, name string) (*models.ProxyConfig, error) {
	if clientID == "" || name == "" {
		return nil, fmt.Errorf("invalid client id or name")
	}
	if err := CanAccessClient(q.ctx, userInfo, clientID, defs.RBACActionView); err != nil {
		return nil, err
	}
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	item := &models.ProxyConfig{}
	err := db.
		Where("tenant_id = ? AND name = ?", userInfo.GetTenantID(), name).
		Where(db.Where("origin_client_id = ?", clientID).Or("client_id = ?", clientID)).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (q *proxyQuery) CountProxyConfigs(userInfo models.UserInfo) (int64, error) {
	return q.CountProxyConfigsWithFilters(userInfo, &models.ProxyConfigEntity{})
}

func (q *proxyQuery) CountProxyConfigsWithFilters(userInfo models.UserInfo, filters *models.ProxyConfigEntity) (int64, error) {
	var count int64
	query, err := q.proxyConfigQuery(userInfo, filters, defs.RBACActionView)
	if err != nil {
		return 0, err
	}
	err = query.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *proxyQuery) CountProxyConfigsWithFiltersAndKeyword(userInfo models.UserInfo, filters *models.ProxyConfigEntity, keyword string) (int64, error) {
	if len(keyword) == 0 {
		return q.CountProxyConfigsWithFilters(userInfo, filters)
	}

	var count int64
	query, err := q.proxyConfigQuery(userInfo, filters, defs.RBACActionView)
	if err != nil {
		return 0, err
	}
	err = query.Where("name like ?", "%"+keyword+"%").Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (q *proxyQuery) GetProxyConfigsByWorkerId(userInfo models.UserInfo, workerID string) ([]*models.ProxyConfig, error) {
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	items := []*models.ProxyConfig{}

	err := db.
		Where(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{
			UserID:   userInfo.GetUserID(),
			TenantID: userInfo.GetTenantID(),
		},
			WorkerID: workerID,
		}).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
