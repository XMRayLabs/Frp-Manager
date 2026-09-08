package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
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
	err := db.
		Where(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			UserID:   userInfo.GetUserID(),
			TenantID: userInfo.GetTenantID(),
			ClientID: clientID,
		}}).
		Or(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			UserID:   0,
			TenantID: userInfo.GetTenantID(),
			ClientID: clientID,
		}}).
		Or(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			UserID:         userInfo.GetUserID(),
			TenantID:       userInfo.GetTenantID(),
			OriginClientID: clientID,
		}}).
		Or(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			UserID:         0,
			TenantID:       userInfo.GetTenantID(),
			OriginClientID: clientID,
		}}).
		Find(&list).Error
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
	db := q.ctx.GetApp().GetDBManager().GetDefaultDB()
	list := []*models.ProxyStats{}
	err := db.
		Where(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
			UserID:   userInfo.GetUserID(),
			TenantID: userInfo.GetTenantID(),
			ServerID: serverID,
		}}).Or(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
		UserID:   0,
		TenantID: userInfo.GetTenantID(),
		ServerID: serverID,
	}}).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return lo.Map(list, func(item *models.ProxyStats, _ int) *models.ProxyStatsEntity {
		return item.ProxyStatsEntity
	}), nil
}

func (m *proxyMutation) AdminUpdateProxyStats(srv *models.ServerEntity, inputs []*pb.ProxyInfo) error {
	if srv.ServerID == "" {
		return fmt.Errorf("invalid server id")
	}

	db := m.ctx.GetApp().GetDBManager().GetDefaultDB()
	return db.Transaction(func(tx *gorm.DB) error {

		queryResults := make([]interface{}, 3)
		p := pool.New().WithErrors()
		p.Go(
			func() error {
				user := models.User{}
				if err := tx.Where(&models.User{
					UserEntity: &models.UserEntity{
						UserID: srv.UserID,
					},
				}).First(&user).Error; err != nil {
					return err
				}
				queryResults[0] = user
				return nil
			},
		)
		p.Go(
			func() error {
				clients := []*models.Client{}
				if err := tx.
					Where(&models.Client{ClientEntity: &models.ClientEntity{
						UserID:   srv.UserID,
						ServerID: srv.ServerID,
					}}).Find(&clients).Error; err != nil {
					return err
				}
				queryResults[1] = clients
				return nil
			},
		)
		p.Go(
			func() error {
				oldProxy := []*models.ProxyStats{}
				if err := tx.
					Where(&models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{
						UserID:   srv.UserID,
						ServerID: srv.ServerID,
					}}).Find(&oldProxy).Error; err != nil {
					return err
				}
				oldProxyMap := lo.SliceToMap(oldProxy, func(p *models.ProxyStats) (string, *models.ProxyStats) {
					return p.Name, p
				})
				queryResults[2] = oldProxyMap
				return nil
			},
		)
		if err := p.Wait(); err != nil {
			return err
		}

		user := queryResults[0].(models.User)
		clients := queryResults[1].([]*models.Client)
		oldProxyMap := queryResults[2].(map[string]*models.ProxyStats)

		inputMap := map[string]*pb.ProxyInfo{}
		proxyMap := map[string]*models.ProxyStatsEntity{}
		for _, proxyInfo := range inputs {
			if proxyInfo == nil {
				continue
			}
			proxyName := strings.TrimPrefix(proxyInfo.GetName(), user.UserName+".")
			proxyMap[proxyName] = &models.ProxyStatsEntity{
				ServerID:        srv.ServerID,
				Name:            proxyName,
				Type:            proxyInfo.GetType(),
				UserID:          srv.UserID,
				TenantID:        srv.TenantID,
				TodayTrafficIn:  proxyInfo.GetTodayTrafficIn(),
				TodayTrafficOut: proxyInfo.GetTodayTrafficOut(),
			}
			inputMap[proxyName] = proxyInfo
		}

		proxyEntityMap := map[string]*models.ProxyStatsEntity{}
		for _, client := range clients {
			cliCfg, err := client.GetConfigContent()
			if err != nil || cliCfg == nil {
				continue
			}
			for _, cfg := range cliCfg.Proxies {
				if proxy, ok := proxyMap[cfg.GetBaseConfig().Name]; ok {
					proxy.ClientID = client.ClientID
					proxy.OriginClientID = client.OriginClientID
					proxyEntityMap[proxy.Name] = proxy
				}
			}
		}

		nowTime := time.Now()
		results := lo.Values(lo.MapValues(proxyEntityMap, func(p *models.ProxyStatsEntity, name string) *models.ProxyStats {
			item := &models.ProxyStats{
				ProxyStatsEntity: p,
			}
			if oldProxy, ok := oldProxyMap[name]; ok {
				item.ProxyID = oldProxy.ProxyID
				firstSync := inputMap[name].GetFirstSync()
				isSameDay := utils.IsSameDay(nowTime, oldProxy.UpdatedAt)

				item.HistoryTrafficIn = oldProxy.HistoryTrafficIn
				item.HistoryTrafficOut = oldProxy.HistoryTrafficOut
				if !isSameDay || firstSync {
					item.HistoryTrafficIn += oldProxy.TodayTrafficIn
					item.HistoryTrafficOut += oldProxy.TodayTrafficOut
				}
			}
			return item
		}))

		if len(results) > 0 {
			return tx.Save(results).Error
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
		var clientIDs, serverIDs []string
		if err := scopeOwnedOrShared(
			db.Model(&models.Client{}),
			q.ctx,
			userInfo,
			defs.RBACObjClient,
			"client_id",
			defs.RBACActionView,
		).Pluck("client_id", &clientIDs).Error; err != nil {
			return nil, err
		}
		if err := scopeOwnedOrShared(
			db.Model(&models.Server{}),
			q.ctx,
			userInfo,
			defs.RBACObjServer,
			"server_id",
			defs.RBACActionView,
		).Pluck("server_id", &serverIDs).Error; err != nil {
			return nil, err
		}

		accessScope := db.Where("user_id = ?", userInfo.GetUserID())
		if len(clientIDs) > 0 {
			accessScope = accessScope.
				Or("client_id IN ?", clientIDs).
				Or("origin_client_id IN ?", clientIDs)
		}
		if len(serverIDs) > 0 {
			accessScope = accessScope.Or("server_id IN ?", serverIDs)
		}
		query = query.Where(accessScope)
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
	} else if !userInfo.IsAdmin() {
		sharedIDs := accessibleObjectIDs(q.ctx, userInfo, defs.RBACObjClient, action)
		ownedScope := db.Where("user_id = ?", userInfo.GetUserID())
		if len(sharedIDs) == 0 {
			query = query.Where(ownedScope)
		} else {
			query = query.Where(
				ownedScope.
					Or("origin_client_id IN ?", sharedIDs).
					Or("client_id IN ?", sharedIDs),
			)
		}
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
