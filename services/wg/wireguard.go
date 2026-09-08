//go:build !windows
// +build !windows

package wg

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
)

func NewWireGuard(ctx *app.Context, ifce defs.WireGuardConfig, logger *logrus.Entry) (app.WireGuard, error) {
	if logger == nil {
		defaultLog := logrus.New()
		logger = logrus.NewEntry(defaultLog)
	}
	cfg := ifce

	if err := InitAndValidateWGConfig(&cfg, logger); err != nil {
		return nil, errors.Join(errors.New("init and validate wg config error"), err)
	}

	svcCtx, cancel := ctx.CopyWithCancel()

	useGvisorNet := ctx.GetApp().GetConfig().App.UseGvisorNet
	if !useGvisorNet {
		useGvisorNet = cfg.GetUseGvisorNet()
	}

	fwManager := newFirewallManager(logger.WithField("component", "iptables"))

	return &wireGuard{
		ifce:             &cfg,
		ctx:              svcCtx,
		cancel:           cancel,
		svcLogger:        logger,
		endpointPingMap:  &utils.SyncMap[uint32, uint32]{},
		useGvisorNet:     useGvisorNet,
		virtAddrPingMap:  &utils.SyncMap[string, uint32]{},
		endpointPingEWMA: make(map[uint32]float64, 64),
		virtAddrPingEWMA: make(map[string]float64, 64),
		fwManager:        fwManager,
		peerDirectory:    make(map[uint32]*pb.WireGuardPeerConfig, 64),
		preconnectPeers:  make(map[uint32]struct{}, 64),
	}, nil
}

// Start implements WireGuard.
func (w *wireGuard) Start() error {
	w.Lock()
	defer w.Unlock()

	log := w.svcLogger.WithField("op", "Start")

	if w.running {
		log.Warnf("wireguard is already running, skip start, ifce: %s", w.ifce.GetInterfaceName())
		return nil
	}

	if err := w.initTransports(); err != nil {
		return errors.Join(fmt.Errorf("init transports failed"), err)
	}

	if err := w.initWGDevice(); err != nil {
		return errors.Join(fmt.Errorf("init WG device failed"), err)
	}

	if err := w.applyPeerConfig(); err != nil {
		return errors.Join(fmt.Errorf("apply peer config failed"), err)
	}

	// 鍦ㄥ簲鐢ㄩ厤缃悗鍐嶅惎鍔ㄨ澶?
	if err := w.wgDevice.Up(); err != nil {
		return errors.Join(fmt.Errorf("wgDevice.Up '%s'", w.ifce.GetInterfaceName()), err)
	}

	if !w.useGvisorNet {
		if err := w.initNetwork(); err != nil {
			return errors.Join(errors.New("init network failed"), err)
		}
	}

	// 鍦?WireGuard 璁惧鍚姩鍚庨厤缃?gvisor
	if w.useGvisorNet {
		if err := w.initGvisorNetwork(); err != nil {
			return errors.Join(errors.New("init gvisor network failed"), err)
		}
	}

	if err := w.applyFirewallRulesLocked(); err != nil {
		return errors.Join(errors.New("apply firewall rules failed"), err)
	}

	log.Infof("Started service done for iface '%s'", w.ifce.GetInterfaceName())
	w.running = true

	go w.reportStatusTask()

	return nil
}

// Stop implements WireGuard.
func (w *wireGuard) Stop() error {
	w.Lock()
	defer w.Unlock()

	log := w.svcLogger.WithField("op", "Stop")
	if !w.running {
		log.Info("Service already down.")
		return nil
	}

	log.Info("Stopping service...")

	if err := w.cleanupFirewallRulesLocked(); err != nil {
		log.WithError(err).Warn("cleanup firewall rules failed")
	}

	w.cleanupWGDevice()
	w.cleanupNetwork()
	w.cancel()

	w.running = false
	log.Info("Service stopped.")
	return nil
}

// AddPeer implements WireGuard.
func (w *wireGuard) AddPeer(peer *defs.WireGuardPeerConfig) error {
	log := w.svcLogger.WithField("op", "AddPeer")

	peerCfg, err := parseAndValidatePeerConfig(peer)
	if err != nil {
		return errors.Join(errors.New("parse and validate peer config"), err)
	}

	w.Lock()
	defer w.Unlock()

	w.ifce.Peers = append(w.ifce.Peers, peer.WireGuardPeerConfig)
	w.indexPeerDirectoryLocked(peerCfg)
	uapiBuilder := NewUAPIBuilder().AddPeerConfig(peerCfg)

	log.Debugf("uapiBuilder: %s", uapiBuilder.Build())

	if err = w.wgDevice.IpcSet(uapiBuilder.Build()); err != nil {
		return errors.Join(errors.New("add peer IpcSet error"), err)
	}

	// 琛ラ綈鏈妭鐐瑰彲杩炴帴鐨勫父椹?peer锛堣嫢鐩綍閲屾湁鍩虹淇℃伅锛?
	w.onPeersChangedLocked("after AddPeer")

	return nil
}

// GenWGConfig implements WireGuard.
func (w *wireGuard) GenWGConfig() (string, error) {
	panic("unimplemented")
}

// GetIfceConfig implements WireGuard.
func (w *wireGuard) GetIfceConfig() (*defs.WireGuardConfig, error) {
	w.RLock()
	defer w.RUnlock()

	return w.ifce, nil
}

// GetBaseIfceConfig implements WireGuard.
func (w *wireGuard) GetBaseIfceConfig() *defs.WireGuardConfig {
	return w.ifce
}

// GetPeer implements WireGuard.
func (w *wireGuard) GetPeer(peerNameOrPk string) (*defs.WireGuardPeerConfig, error) {
	w.RLock()
	defer w.RUnlock()

	for _, p := range w.ifce.Peers {
		if p.ClientId == peerNameOrPk || p.PublicKey == peerNameOrPk {
			return &defs.WireGuardPeerConfig{WireGuardPeerConfig: p}, nil
		}
	}
	return nil, errors.New("peer not found")
}

// ListPeers implements WireGuard.
func (w *wireGuard) ListPeers() ([]*defs.WireGuardPeerConfig, error) {
	w.RLock()
	defer w.RUnlock()

	return w.ifce.GetParsedPeers(), nil
}

// RemovePeer implements WireGuard.
func (w *wireGuard) RemovePeer(peerNameOrPk string) error {
	log := w.svcLogger.WithField("op", "RemovePeer")

	w.Lock()
	defer w.Unlock()

	// 璇箟锛氱湡姝ｇЩ闄?peer锛堜笅鍙?remove=true锛夛紝骞朵粠鏈湴閰嶇疆涓垹闄ゃ€?
	// 濡傞渶瑕佲€滃彧绉婚櫎璺敱浣嗕繚鎸佽繛鎺モ€濓紝璇蜂娇鐢?PatchPeers/UpdatePeer 涓嬪彂 AllowedIPs=nil 鐨勬洿鏂扮瓥鐣ャ€?

	var removedPeerPB *pb.WireGuardPeerConfig
	newPeers := make([]*pb.WireGuardPeerConfig, 0, len(w.ifce.Peers))
	for _, p := range w.ifce.Peers {
		if p.ClientId == peerNameOrPk || p.PublicKey == peerNameOrPk {
			removedPeerPB = p
			continue
		}
		newPeers = append(newPeers, p)
	}

	if removedPeerPB == nil {
		return errors.New("peer not found")
	}

	removedPeer := &defs.WireGuardPeerConfig{WireGuardPeerConfig: removedPeerPB}
	log.Debugf("remove peer completely: key=%s pk=%s", truncate(peerNameOrPk, 10), truncate(removedPeer.GetPublicKey(), 10))

	uapiBuilder := NewUAPIBuilder().RemovePeerByKey(removedPeer.GetParsedPublicKey())

	log.Debugf("uapiBuilder: %s", uapiBuilder.Build())

	if err := w.wgDevice.IpcSet(uapiBuilder.Build()); err != nil {
		return errors.Join(errors.New("remove peer IpcSet error"), err)
	}

	// IpcSet 鎴愬姛鍚庡啀鏇存柊鏈湴缂撳瓨锛岄伩鍏嶄笉涓€鑷?
	w.ifce.Peers = newPeers
	w.deletePeerDirectoryLocked(removedPeer)
	if id := removedPeer.GetId(); id != 0 {
		delete(w.preconnectPeers, id)
	}
	if removedPeer.GetEndpoint() != nil {
		if id := removedPeer.GetEndpoint().GetWireguardId(); id != 0 {
			delete(w.preconnectPeers, id)
		}
	}
	w.cleanupPreconnectPeersLocked()

	// 绉婚櫎鍚庝篃琛ラ綈鍏朵粬鍙繛鎺?peer锛堣嫢鐩綍閲屾湁鍩虹淇℃伅锛?
	w.onPeersChangedLocked("after RemovePeer")

	return nil
}

// UpdatePeer implements WireGuard.
func (w *wireGuard) UpdatePeer(peer *defs.WireGuardPeerConfig) error {
	log := w.svcLogger.WithField("op", "UpdatePeer")

	peerCfg, err := parseAndValidatePeerConfig(peer)
	if err != nil {
		return errors.Join(errors.New("parse and validate peer config"), err)
	}

	w.Lock()
	defer w.Unlock()

	newPeers := []*pb.WireGuardPeerConfig{}
	for _, p := range w.ifce.Peers {
		if p.ClientId != peer.ClientId && p.PublicKey != peer.PublicKey {
			newPeers = append(newPeers, p)
			continue
		}
		newPeers = append(newPeers, peer.WireGuardPeerConfig)
	}

	w.ifce.Peers = newPeers
	w.indexPeerDirectoryLocked(peerCfg)

	uapiBuilder := NewUAPIBuilder().UpdatePeerConfig(peerCfg)

	log.Debugf("uapiBuilder: %s", uapiBuilder.Build())

	if err := w.wgDevice.IpcSet(uapiBuilder.Build()); err != nil {
		return errors.Join(errors.New("update peer IpcSet error"), err)
	}

	// 鏇存柊鍚庤ˉ榻愬父椹?peer锛堣嫢鐩綍閲屾湁鍩虹淇℃伅锛?
	w.onPeersChangedLocked("after UpdatePeer")

	return nil
}

func (w *wireGuard) PatchPeers(newPeers []*defs.WireGuardPeerConfig) (*app.WireGuardDiffPeersResponse, error) {
	log := w.svcLogger.WithField("op", "PatchPeers")

	// 閲嶈锛氫笉瑕佷娇鐢?utils.Diff 鐩存帴瀵?Equal 鍋?diff銆?
	// Equal() 鍖呭惈 AllowedIPs/Endpoint 绛夊瓧娈碉紝浠讳綍璺敱鍙樺寲閮戒細琚綋浣?remove+add锛?
	// 鍐嶅彔鍔犲綋鍓嶁€滃厛鍒犲悗鍔犫€濈殑鎵ц椤哄簭锛屼細瀵艰嚧鐭椂鏂摼/璺敱榛戞礊銆?
	// 鎸?PublicKey 鍋氱ǔ瀹氬尮閰嶏紝灏嗗彉鍖栧綊绫讳负 add/update/remove

	oldPeers := w.ifce.GetParsedPeers()
	typedNewPeers, err := parseAndValidatePeerConfigs(newPeers)
	if err != nil {
		return nil, err
	}

	// 浼樺寲锛氫粠 adj 涓彁鍙栤€滄湰鑺傜偣鍙洿杩?鍙繛鎺モ€濈殑 peer锛岀‘淇濆畠浠父椹讳絾涓嶅垎閰?AllowedIPs銆?
	// 杩欐牱鍚庣画璺敱鍙樺寲鍙渶瑕佹洿鏂?AllowedIPs锛屼笉闇€瑕?remove+add 閲嶆柊寤虹珛杩炴帴銆?
	beforeMerge := len(typedNewPeers)
	typedNewPeers = mergeConnectablePeersFromAdj(w.ifce, typedNewPeers, oldPeers)
	if delta := len(typedNewPeers) - beforeMerge; delta > 0 {
		log.Debugf("merged connectable peers from adjs: +%d (desired=%d -> %d)", delta, beforeMerge, len(typedNewPeers))
	}

	// 鏇存柊 peerDirectory锛堟妸鏈鐪嬪埌鐨?peer 鍩虹淇℃伅閮界紦瀛樿捣鏉ワ紝鍚庣画 adjs 鍙樺寲鍙敤浜庤ˉ榻愬父椹?peer锛?
	w.Lock()
	for _, p := range typedNewPeers {
		if p == nil {
			continue
		}
		w.indexPeerDirectoryLocked(p)
	}
	w.Unlock()

	oldByPK := make(map[string]*defs.WireGuardPeerConfig, len(oldPeers))
	for _, p := range oldPeers {
		if p == nil || p.GetPublicKey() == "" {
			continue
		}
		oldByPK[p.GetPublicKey()] = p
	}

	newByPK := make(map[string]*defs.WireGuardPeerConfig, len(typedNewPeers))
	for _, p := range typedNewPeers {
		if p == nil || p.GetPublicKey() == "" {
			continue
		}
		newByPK[p.GetPublicKey()] = p
	}

	addPeers := make([]*defs.WireGuardPeerConfig, 0, 8)
	updatePeers := make([]*defs.WireGuardPeerConfig, 0, 8)
	removePeers := make([]*defs.WireGuardPeerConfig, 0, 8)

	for pk, np := range newByPK {
		op, ok := oldByPK[pk]
		if !ok { // new peer
			addPeers = append(addPeers, np)
			continue
		}
		if !op.Equal(np) { // update peer
			updatePeers = append(updatePeers, np)
		}
	}
	for pk, op := range oldByPK {
		if _, ok := newByPK[pk]; !ok { // remove peer
			removePeers = append(removePeers, op)
		}
	}

	resp := &app.WireGuardDiffPeersResponse{
		AddPeers:    addPeers,
		RemovePeers: removePeers,
	}

	if len(addPeers) == 0 && len(updatePeers) == 0 && len(removePeers) == 0 {
		return resp, nil
	}

	uapiBuilder := NewUAPIBuilder()
	for _, p := range addPeers {
		uapiBuilder.AddPeerConfig(p)
	}
	for _, p := range updatePeers {
		uapiBuilder.UpdatePeerConfig(p)
	}
	for _, p := range removePeers {
		uapiBuilder.RemovePeerByKey(p.GetParsedPublicKey())
	}

	log.Debugf("uapiBuilder: %s", uapiBuilder.Build())

	w.Lock()
	defer w.Unlock()

	if err := w.wgDevice.IpcSet(uapiBuilder.Build()); err != nil {
		return nil, errors.Join(errors.New("patch peers IpcSet error"), err)
	}

	// IpcSet 鎴愬姛鍚庡啀鏇存柊鏈湴缂撳瓨锛岄伩鍏嶄笉涓€鑷?
	newPBPeers := make([]*pb.WireGuardPeerConfig, 0, len(typedNewPeers))
	for _, p := range typedNewPeers {
		if p == nil || p.WireGuardPeerConfig == nil {
			continue
		}
		newPBPeers = append(newPBPeers, p.WireGuardPeerConfig)
	}
	w.ifce.Peers = newPBPeers

	// 娓呯悊 preconnectPeers 涓凡涓嶅瓨鍦ㄧ殑 peer id锛岄伩鍏嶆棤闄愬闀?
	w.cleanupPreconnectPeersLocked()

	// PatchPeers 鍚庡啀娆¤ˉ榻愶紙涓昏瑕嗙洊锛歛djs 鍏堜簬 peers 鍙樺寲銆佷笖鐩綍宸叉湁淇℃伅鐨勫満鏅級
	w.onPeersChangedLocked("after PatchPeers")

	return resp, nil
}

func (w *wireGuard) GetWGRuntimeInfo() (*pb.WGDeviceRuntimeInfo, error) {
	runningInfo, err := w.wgDevice.IpcGet()
	if err != nil {
		return nil, errors.Join(errors.New("get WG running info error"), err)
	}

	runtimeInfo, err := ParseWGRunningInfo(runningInfo)
	if err != nil {
		return nil, err
	}

	runtimeInfo.PingMap = w.endpointPingMap.Export()
	runtimeInfo.VirtAddrPingMap = w.virtAddrPingMap.Export()

	if w.useGvisorNet {
		runtimeInfo.InterfaceName = w.ifce.GetInterfaceName()
	} else {
		link, err := netlink.LinkByName(w.ifce.GetInterfaceName())
		if err != nil {
			return nil, errors.Join(fmt.Errorf("get iface '%s' via netlink", w.ifce.GetInterfaceName()), err)
		}
		runtimeInfo.InterfaceName = link.Attrs().Name
	}

	parsedPeers := w.ifce.GetParsedPeers()
	parsedPublicKeysPeerMap := make(map[string]*defs.WireGuardPeerConfig)
	for _, peer := range parsedPeers {
		parsedPublicKeysPeerMap[peer.HexPublicKey()] = peer
	}

	runtimeInfo.PeerVirtAddrMap = make(map[string]uint32)
	for _, peer := range parsedPeers {
		runtimeInfo.PeerVirtAddrMap[peer.GetVirtualIp()] = peer.GetId()
	}

	for _, peerRuntimeInfo := range runtimeInfo.GetPeers() {
		peerConfig, ok := parsedPublicKeysPeerMap[peerRuntimeInfo.PublicKey]
		if !ok {
			continue
		}
		if runtimeInfo.PeerConfigMap == nil {
			runtimeInfo.PeerConfigMap = make(map[string]*pb.WireGuardPeerConfig)
		}
		runtimeInfo.PeerConfigMap[peerRuntimeInfo.PublicKey] = peerConfig.WireGuardPeerConfig
		peerRuntimeInfo.ClientId = peerConfig.GetClientId()
	}

	return runtimeInfo, nil
}

func (w *wireGuard) UpdateAdjs(adjs map[uint32]*pb.WireGuardLinks) error {
	w.Lock()
	defer w.Unlock()

	w.ifce.Adjs = adjs

	// adjs 鍙樺寲鍚庣珛鍒昏ˉ榻愨€滃彲杩炴帴 peer 甯搁┗鈥濓紙鍙 peerDirectory 涓凡鏈夊叾鍩虹淇℃伅锛?
	return w.ensureConnectablePeersLocked()
}

func (w *wireGuard) NeedRecreate(newCfg *defs.WireGuardConfig) bool {
	return w.ifce.GetId() != newCfg.GetId() ||
		w.ifce.GetInterfaceName() != newCfg.GetInterfaceName() ||
		w.ifce.GetPrivateKey() != newCfg.GetPrivateKey() ||
		w.ifce.GetLocalAddress() != newCfg.GetLocalAddress() ||
		w.ifce.GetListenPort() != newCfg.GetListenPort() ||
		w.ifce.GetWsListenPort() != newCfg.GetWsListenPort() ||
		w.ifce.GetInterfaceMtu() != newCfg.GetInterfaceMtu() ||
		w.ifce.GetUseGvisorNet() != newCfg.GetUseGvisorNet() ||
		w.ifce.GetNetworkId() != newCfg.GetNetworkId()
}
