package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/samber/lo"
)

const (
	proxyStatusCacheTTL     = 10 * time.Second
	proxyStatusRPCTimeout   = 2500 * time.Millisecond
	proxyStatusLegacyLimit  = 5 * time.Second
	proxyStatusRPCWorkers   = 16
	maxProxyStatusBatchSize = 10000
)

type proxyStatusCacheEntry struct {
	status    *pb.ProxyWorkingStatus
	expiresAt time.Time
}

var proxyStatusCache sync.Map

func GetProxyStatuses(ctx *app.Context, req *pb.GetProxyStatusesRequest) (*pb.GetProxyStatusesResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	if !userInfo.Valid() {
		return &pb.GetProxyStatusesResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}
	if len(req.GetProxyIds()) > maxProxyStatusBatchSize {
		return nil, fmt.Errorf("too many proxy ids: maximum is %d", maxProxyStatusBatchSize)
	}

	filter := &models.ProxyConfigEntity{}
	if req.GetClientId() != "" {
		filter.OriginClientID = req.GetClientId()
	}
	if req.GetServerId() != "" {
		filter.ServerID = req.GetServerId()
	}

	var (
		configs []*models.ProxyConfig
		err     error
	)
	if len(req.GetProxyIds()) > 0 {
		configs, err = dao.NewQuery(ctx).ListProxyConfigsByIDs(userInfo, req.GetProxyIds())
	} else if req.GetKeyword() != "" {
		configs, err = dao.NewQuery(ctx).ListProxyConfigsWithFiltersAndKeyword(
			userInfo, 1, maxProxyStatusBatchSize, filter, req.GetKeyword(),
		)
	} else {
		configs, err = dao.NewQuery(ctx).ListProxyConfigsWithFilters(
			userInfo, 1, maxProxyStatusBatchSize, filter,
		)
	}
	if err != nil {
		return nil, err
	}

	results := collectProxyStatuses(ctx, configs)
	return &pb.GetProxyStatusesResponse{
		Status:        &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		ProxyStatuses: results,
	}, nil
}

func collectProxyStatuses(ctx *app.Context, configs []*models.ProxyConfig) []*pb.ProxyStatusResult {
	results := make([]*pb.ProxyStatusResult, 0, len(configs))
	groups := make(map[string][]*models.ProxyConfig)
	now := time.Now()

	for _, config := range configs {
		if config == nil || config.ID == 0 {
			continue
		}
		proxyID := uint32(config.ID)
		if config.Stopped {
			status := proxyStatus("stopped", "")
			storeProxyStatus(proxyID, status)
			results = append(results, proxyStatusResult(proxyID, status))
			continue
		}
		if status, ok := loadProxyStatus(proxyID, now); ok {
			results = append(results, proxyStatusResult(proxyID, status))
			continue
		}

		originClientID := config.OriginClientID
		if originClientID == "" {
			originClientID = config.ClientID
		}
		groups[originClientID] = append(groups[originClientID], config)
	}

	if len(groups) == 0 {
		return results
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		workers = make(chan struct{}, proxyStatusRPCWorkers)
	)
	for originClientID, group := range groups {
		originClientID, group := originClientID, group
		wg.Add(1)
		go func() {
			defer wg.Done()
			workers <- struct{}{}
			defer func() { <-workers }()

			groupResults := refreshProxyStatusGroup(ctx, originClientID, group)
			mu.Lock()
			results = append(results, groupResults...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}

func refreshProxyStatusGroup(
	ctx *app.Context,
	originClientID string,
	configs []*models.ProxyConfig,
) []*pb.ProxyStatusResult {
	targets := make([]*pb.ProxyStatusTarget, 0, len(configs))
	for _, config := range configs {
		targets = append(targets, &pb.ProxyStatusTarget{
			ProxyId:  lo.ToPtr(uint32(config.ID)),
			ClientId: lo.ToPtr(config.ClientID),
			ServerId: lo.ToPtr(config.ServerID),
			Name:     lo.ToPtr(config.Name),
		})
	}

	rpcContext, cancel := context.WithTimeout(ctx.Context, proxyStatusRPCTimeout)
	defer cancel()
	rpcAppContext := app.NewContext(rpcContext, ctx.GetApp())
	rpcResponse := &pb.GetProxyStatusesRPCResponse{}
	err := rpc.CallClientWrapper(
		rpcAppContext,
		originClientID,
		pb.Event_EVENT_GET_PROXY_INFO_BATCH,
		&pb.GetProxyStatusesRPCRequest{Targets: targets},
		rpcResponse,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown event") {
			ctx.Logger().Infof(
				"client [%s] uses the legacy proxy status protocol; using one-request compatibility mode",
				originClientID,
			)
			return refreshLegacyProxyStatusGroup(ctx, originClientID, configs)
		}
		ctx.Logger().WithError(err).Warnf(
			"batch proxy status query failed for client [%s], proxies: %d",
			originClientID,
			len(configs),
		)
		results := make([]*pb.ProxyStatusResult, 0, len(configs))
		for _, config := range configs {
			proxyID := uint32(config.ID)
			status := proxyStatus("error", err.Error())
			storeProxyStatus(proxyID, status)
			results = append(results, proxyStatusResult(proxyID, status))
		}
		return results
	}

	received := make(map[uint32]*pb.ProxyWorkingStatus, len(rpcResponse.GetProxyStatuses()))
	for _, result := range rpcResponse.GetProxyStatuses() {
		if result != nil && result.GetProxyId() != 0 {
			received[result.GetProxyId()] = result.GetWorkingStatus()
		}
	}

	// Some much older kernels use user-prefixed status names. Retry only a
	// confirmed lookup miss; never replace a real runtime error or timeout.
	var missing []*models.ProxyConfig
	for _, config := range configs {
		if proxyStatusNameMissing(received[uint32(config.ID)]) {
			missing = append(missing, config)
		}
	}
	if len(missing) > 0 {
		prefixes := legacyProxyStatusPrefixes(ctx, missing)
		var compatibilityTargets []*pb.ProxyStatusTarget
		for _, config := range missing {
			if prefix := prefixes[config.ClientID]; prefix != "" {
				compatibilityTargets = append(compatibilityTargets, &pb.ProxyStatusTarget{
					ProxyId: lo.ToPtr(uint32(config.ID)), ClientId: lo.ToPtr(config.ClientID),
					ServerId: lo.ToPtr(config.ServerID), Name: lo.ToPtr(prefix + config.Name),
				})
			}
		}
		if len(compatibilityTargets) > 0 {
			compatibilityResponse := &pb.GetProxyStatusesRPCResponse{}
			if err := rpc.CallClientWrapper(rpcAppContext, originClientID, pb.Event_EVENT_GET_PROXY_INFO_BATCH,
				&pb.GetProxyStatusesRPCRequest{Targets: compatibilityTargets}, compatibilityResponse); err == nil {
				for _, result := range compatibilityResponse.GetProxyStatuses() {
					if result != nil && proxyStatusNameMissing(received[result.GetProxyId()]) && result.GetWorkingStatus() != nil {
						received[result.GetProxyId()] = result.GetWorkingStatus()
					}
				}
			}
		}
	}
	results := make([]*pb.ProxyStatusResult, 0, len(configs))
	for _, config := range configs {
		proxyID := uint32(config.ID)
		status := received[proxyID]
		if status == nil || status.GetStatus() == "" {
			status = proxyStatus("unknown", "")
		}
		storeProxyStatus(proxyID, status)
		results = append(results, proxyStatusResult(proxyID, status))
	}
	return results
}

func refreshLegacyProxyStatusGroup(
	ctx *app.Context,
	originClientID string,
	configs []*models.ProxyConfig,
) []*pb.ProxyStatusResult {
	results := make([]*pb.ProxyStatusResult, len(configs))
	workers := make(chan struct{}, proxyStatusRPCWorkers)
	var prefixOnce sync.Once
	var prefixes map[string]string
	var wg sync.WaitGroup
	legacyContext, cancel := context.WithTimeout(ctx.Context, proxyStatusLegacyLimit)
	defer cancel()

	for index, config := range configs {
		index, config := index, config
		wg.Add(1)
		go func() {
			defer wg.Done()
			workers <- struct{}{}
			defer func() { <-workers }()

			response := &pb.GetProxyConfigResponse{}
			err := rpc.CallClientWrapper(
				app.NewContext(legacyContext, ctx.GetApp()),
				originClientID,
				pb.Event_EVENT_GET_PROXY_INFO,
				&pb.GetProxyConfigRequest{
					ClientId: lo.ToPtr(config.ClientID),
					ServerId: lo.ToPtr(config.ServerID),
					Name:     lo.ToPtr(config.Name),
				},
				response,
			)

			// Legacy protocols can fail the whole RPC for an individual miss.
			if (err != nil && strings.TrimSpace(err.Error()) == "client return error: cannot get proxy status") ||
				(err == nil && proxyStatusNameMissing(response.GetWorkingStatus())) {
				prefixOnce.Do(func() { prefixes = legacyProxyStatusPrefixes(ctx, configs) })
				if prefix := prefixes[config.ClientID]; prefix != "" {
					err = rpc.CallClientWrapper(app.NewContext(legacyContext, ctx.GetApp()), originClientID,
						pb.Event_EVENT_GET_PROXY_INFO, &pb.GetProxyConfigRequest{
							ClientId: lo.ToPtr(config.ClientID), ServerId: lo.ToPtr(config.ServerID), Name: lo.ToPtr(prefix + config.Name),
						}, response)
				}
			}
			status := response.GetWorkingStatus()
			if err != nil {
				status = proxyStatus("error", err.Error())
			} else if status == nil || status.GetStatus() == "" {
				status = proxyStatus("unknown", "")
			}
			proxyID := uint32(config.ID)
			storeProxyStatus(proxyID, status)
			results[index] = proxyStatusResult(proxyID, status)
		}()
	}
	wg.Wait()
	return results
}

func loadProxyStatus(proxyID uint32, now time.Time) (*pb.ProxyWorkingStatus, bool) {
	value, ok := proxyStatusCache.Load(proxyID)
	if !ok {
		return nil, false
	}
	entry, ok := value.(proxyStatusCacheEntry)
	if !ok || now.After(entry.expiresAt) {
		proxyStatusCache.Delete(proxyID)
		return nil, false
	}
	return entry.status, true
}

func storeProxyStatus(proxyID uint32, status *pb.ProxyWorkingStatus) {
	proxyStatusCache.Store(proxyID, proxyStatusCacheEntry{
		status:    status,
		expiresAt: time.Now().Add(proxyStatusCacheTTL),
	})
}

func proxyStatus(status, errMessage string) *pb.ProxyWorkingStatus {
	return &pb.ProxyWorkingStatus{
		Status: lo.ToPtr(status),
		Err:    lo.ToPtr(errMessage),
	}
}

func proxyStatusResult(proxyID uint32, status *pb.ProxyWorkingStatus) *pb.ProxyStatusResult {
	return &pb.ProxyStatusResult{
		ProxyId:       lo.ToPtr(proxyID),
		WorkingStatus: status,
	}
}

// Overview checks run in the background with bounded concurrency. Cached results
// remain visible during refresh, so large fleets never block the dashboard on RPC.
var overviewProbeSlots = make(chan struct{}, 8)
var overviewProbes sync.Map

func OverviewProxyStatuses(ctx *app.Context, configs []*models.ProxyConfig) []*pb.ProxyStatusResult {
	results := make([]*pb.ProxyStatusResult, 0, len(configs))
	groups := map[string][]*models.ProxyConfig{}
	for _, cfg := range configs {
		if entry, ok := proxyStatusCache.Load(uint32(cfg.ID)); ok {
			if cached, ok := entry.(proxyStatusCacheEntry); ok {
				results = append(results, proxyStatusResult(uint32(cfg.ID), cached.status))
				if time.Now().Before(cached.expiresAt.Add(20 * time.Second)) {
					continue
				}
			}
		}
		id := cfg.OriginClientID
		if id == "" {
			id = cfg.ClientID
		}
		groups[id] = append(groups[id], cfg)
	}
	for key, group := range groups {
		if _, busy := overviewProbes.LoadOrStore(key, true); busy {
			continue
		}
		select {
		case overviewProbeSlots <- struct{}{}:
		default:
			overviewProbes.Delete(key)
			continue
		}
		go func(key string, group []*models.ProxyConfig) {
			defer func() { <-overviewProbeSlots; overviewProbes.Delete(key) }()
			probeCtx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			refreshProxyStatusGroup(app.NewContext(probeCtx, ctx.GetApp()), key, group)
		}(key, group)
	}
	return results
}

func proxyStatusNameMissing(status *pb.ProxyWorkingStatus) bool {
	return status != nil && status.GetStatus() == "error" && strings.TrimSpace(status.GetErr()) == "cannot get proxy status"
}

// Compatibility prefixes come from each tunnel's configured FRP user, never
// from the administrator or group member viewing the dashboard.
func legacyProxyStatusPrefixes(ctx *app.Context, configs []*models.ProxyConfig) map[string]string {
	prefixes := make(map[string]string, len(configs))
	ids := make([]string, 0, len(configs))
	for _, config := range configs {
		ids = append(ids, config.ClientID)
	}
	var clients []models.ClientEntity
	if err := ctx.GetApp().GetDBManager().GetDefaultDB().Table("clients").
		Select("client_id, config_content").Where("client_id IN ? AND deleted_at IS NULL", ids).Find(&clients).Error; err != nil {
		ctx.Logger().WithError(err).Warn("cannot read configured proxy status prefixes")
		return prefixes
	}
	for _, client := range clients {
		var common struct {
			User string `json:"user"`
		}
		if json.Unmarshal(client.ConfigContent, &common) == nil && common.User != "" {
			prefixes[client.ClientID] = common.User + "."
		}
	}
	return prefixes
}
