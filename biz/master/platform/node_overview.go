package platform

import (
	"net/http"
	"strings"

	proxyapi "github.com/Sakurame1/frp-manager/biz/master/proxy"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/policy/security"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/util/version"
)

type nodeOverview struct {
	PendingIDs   []string `json:"pendingIds"`
	Total        int      `json:"total"`
	Online       int      `json:"online"`
	Pending      int      `json:"pending"`
	Unconfigured int      `json:"unconfigured"`
	Invalid      int      `json:"invalid"`
	Unavailable  int      `json:"unavailable"`
	Upgrade      int      `json:"upgrade"`
}

type overviewResponse struct {
	Clients nodeOverview `json:"clients"`
	Servers nodeOverview `json:"servers"`
}

func GetNodeOverview(instance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := app.NewContext(c, instance)
		if !common.GetUserInfo(ctx).Valid() {
			common.ErrUnAuthorized(c, "invalid user")
			return
		}
		result, err := getNodeOverview(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Err(err.Error()))
			return
		}
		c.JSON(http.StatusOK, common.OK(defs.ReqSuccess).WithBody(result))
	}
}

// Reasons overlap, but a node contributes at most once to Pending.
func (n *nodeOverview) add(online, unconfigured, invalid, unavailable, upgrade bool) {
	n.Total++
	if online {
		n.Online++
	}
	if unconfigured {
		n.Unconfigured++
	}
	if invalid {
		n.Invalid++
	}
	if unavailable {
		n.Unavailable++
	}
	if upgrade {
		n.Upgrade++
	}
	if unconfigured || invalid || unavailable || upgrade {
		n.Pending++
	}
}

func needsNodeUpgrade(installed, current string) bool {
	if strings.EqualFold(strings.TrimSpace(installed), "main") {
		return true
	}
	old, err := version.ParseSemantic(strings.TrimSpace(installed))
	if err != nil {
		return false
	}
	latest, err := version.ParseSemantic(strings.TrimSpace(current))
	if err != nil {
		return false
	}
	return old.LessThan(latest)
}

func nodeConnection(ctx *app.Context, id, kind string) (online, unhealthy, upgrade bool) {
	mgr := ctx.GetApp().GetClientsManager()
	connector := mgr.Get(id)
	if connector == nil || connector.CliType != kind {
		return false, false, false
	}
	online = true
	if snapshot, ok := mgr.GetRuntimeSnapshot(id); ok {
		unhealthy = !snapshot.Healthy
		upgrade = needsNodeUpgrade(snapshot.Version.GetGitVersion(), conf.GetVersion().GitVersion)
	}
	scheduleStatusProbe(ctx.GetApp(), id, connector)
	return
}

func serverConfigInvalid(s *models.ServerEntity) bool {
	cfg, err := s.GetConfigContent()
	if err != nil || cfg == nil {
		return true
	}
	if strings.TrimSpace(s.ServerIP) == "" && len(s.FrpsUrls) == 0 {
		return true
	}
	if err = cfg.Complete(); err != nil {
		return true
	}
	_, err = validation.NewConfigValidator(security.NewUnsafeFeatures(nil)).ValidateServerConfig(cfg)
	return err != nil
}

func getNodeOverview(ctx *app.Context) (*overviewResponse, error) {
	user := common.GetUserInfo(ctx)
	q := dao.NewQuery(ctx)
	result := &overviewResponse{}
	clients := map[string]*models.ClientEntity{}
	// Use the same visibility and temporary-node rules as the node lists, without
	// a page-size ceiling or exposing node secrets/configuration to the browser.
	for page := 1; ; page++ {
		rows, err := q.ListClients(user, page, 500)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			clients[row.ClientID] = row
		}
		if len(rows) < 500 {
			break
		}
	}
	servers := map[string]*models.ServerEntity{}
	serverBad := map[string]bool{}
	serverOnline := map[string]bool{}
	for page := 1; ; page++ {
		rows, err := q.ListServers(user, page, 500)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			servers[row.ServerID] = row
			missing := len(strings.TrimSpace(string(row.ConfigContent))) == 0
			invalid := !missing && serverConfigInvalid(row)
			online, unhealthy, upgrade := nodeConnection(ctx, row.ServerID, defs.CliTypeServer)
			serverOnline[row.ServerID] = online
			serverBad[row.ServerID] = missing || invalid || (online && unhealthy)
			result.Servers.add(online, missing, invalid, online && unhealthy, upgrade)
			if missing || invalid || (online && unhealthy) || upgrade {
				result.Servers.PendingIDs = append(result.Servers.PendingIDs, row.ServerID)
			}
		}
		if len(rows) < 500 {
			break
		}
	}
	all, err := q.GetAllClients(user)
	if err != nil {
		return nil, err
	}
	prefixes := map[string]string{}
	for _, row := range all {
		if len(row.ConfigContent) == 0 {
			continue
		}
		if cfg, err := row.GetConfigContent(); err == nil {
			prefixes[row.ClientID] = cfg.User
		}
	}
	configured := map[string]bool{}
	invalid := map[string]bool{}
	failed := map[string]bool{}
	for page := 1; ; page++ {
		rows, err := q.ListProxyConfigsWithFilters(user, page, 128, &models.ProxyConfigEntity{})
		if err != nil {
			return nil, err
		}
		active := make([]*models.ProxyConfig, 0, len(rows))
		owners := map[uint32]string{}
		for _, row := range rows {
			id := row.OriginClientID
			if id == "" {
				id = row.ClientID
			}
			client := clients[id]
			if client == nil {
				continue
			}
			configured[id] = true
			cfg, err := row.GetTypedProxyConfig()
			if err != nil || cfg.ProxyConfigurer == nil || servers[row.ServerID] == nil {
				invalid[id] = true
				continue
			}
			cfg.Complete()
			err = validation.ValidateProxyConfigurerForClient(cfg.ProxyConfigurer)
			if err != nil {
				invalid[id] = true
			}
			if row.Stopped || client.Stopped {
				continue
			}
			if serverBad[row.ServerID] {
				failed[id] = true
			}
			// A disconnected endpoint is normal; do not interpret its cached
			// proxy failures as actionable runtime errors.
			if !serverOnline[row.ServerID] || ctx.GetApp().GetClientsManager().Get(id) == nil {
				continue
			}
			owners[uint32(row.ID)] = id
			active = append(active, row)
		}
		// Existing batched RPC and status cache avoid a separate request per proxy.
		for _, state := range proxyapi.OverviewProxyStatuses(ctx, prefixes, active) {
			status := state.GetWorkingStatus()
			if status.GetErr() != "" || (status.GetStatus() != "running" && status.GetStatus() != "stopped") {
				failed[owners[state.GetProxyId()]] = true
			}
		}
		if len(rows) < 128 {
			break
		}
	}
	// A client may be configured directly (including visitor-only configurations).
	// Child configurations belong to one visible physical node.
	for _, row := range all {
		id := row.OriginClientID
		if id == "" {
			id = row.ClientID
		}
		if clients[id] == nil || row.IsShadow || len(strings.TrimSpace(string(row.ConfigContent))) == 0 {
			continue
		}
		cfg, err := row.GetConfigContent()
		if err != nil {
			invalid[id] = true
			continue
		}
		if _, err := validation.NewConfigValidator(security.NewUnsafeFeatures(nil)).ValidateClientCommonConfig(&cfg.ClientCommonConfig); err != nil {
			invalid[id] = true
		}
		if cfg.ServerAddr == "" || cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
			invalid[id] = true
		}
		for _, proxy := range cfg.Proxies {
			if proxy.ProxyConfigurer == nil {
				invalid[id] = true
				continue
			}
			proxy.Complete()
			if err := validation.ValidateProxyConfigurerForClient(proxy.ProxyConfigurer); err != nil {
				invalid[id] = true
			}
		}
		for _, visitor := range cfg.Visitors {
			if visitor.VisitorConfigurer == nil {
				invalid[id] = true
				continue
			}
			visitor.Complete()
			if err := validation.ValidateVisitorConfigurer(visitor.VisitorConfigurer); err != nil {
				invalid[id] = true
			}
		}
		if len(cfg.Proxies) > 0 || len(cfg.Visitors) > 0 {
			configured[id] = true
		}
	}
	for id, row := range clients {
		online, unhealthy, upgrade := nodeConnection(ctx, id, defs.CliTypeClient)
		result.Clients.add(online, !configured[id], invalid[id], online && !row.Stopped && (unhealthy || failed[id]), upgrade)
		if !configured[id] || invalid[id] || (online && !row.Stopped && (unhealthy || failed[id])) || upgrade {
			result.Clients.PendingIDs = append(result.Clients.PendingIDs, id)
		}
	}
	return result, nil
}
