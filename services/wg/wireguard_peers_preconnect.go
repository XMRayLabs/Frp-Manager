//go:build !windows
// +build !windows

package wg

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
)

// peer 棰勮繛鎺?甯搁┗琛ラ綈閫昏緫
//
// 鐩爣锛氬湪鎷撴墤(adjs)鍙樺寲鏃讹紝纭繚鏈妭鐐瑰鈥滃彲鐩磋繛/鍙繛鎺モ€濈殑 peer 宸查厤缃埌 wg 璁惧锛?
// 浣?AllowedIPs 涓虹┖锛堝彧淇濇寔杩炴帴锛屼笉鎵胯浇璺敱锛夛紝浠庤€岄伩鍏嶈矾鐢卞彉鍖栧鑷撮绻?remove+add 閫犳垚鏂摼銆?

func (w *wireGuard) onPeersChangedLocked(reason string) {
	if w == nil {
		return
	}
	if err := w.ensureConnectablePeersLocked(); err != nil {
		w.svcLogger.WithError(err).WithField("op", "onPeersChanged").Warnf("ensure connectable peers failed (%s)", reason)
	}
}

func (w *wireGuard) cleanupPreconnectPeersLocked() {
	if w == nil {
		return
	}
	if len(w.preconnectPeers) == 0 {
		return
	}
	exists := make(map[uint32]struct{}, len(w.ifce.Peers))
	for _, p := range w.ifce.GetParsedPeers() {
		if p == nil {
			continue
		}
		if id := p.GetId(); id != 0 {
			exists[id] = struct{}{}
		}
		if p.GetEndpoint() != nil {
			if id := p.GetEndpoint().GetWireguardId(); id != 0 {
				exists[id] = struct{}{}
			}
		}
	}
	for id := range w.preconnectPeers {
		if _, ok := exists[id]; !ok {
			delete(w.preconnectPeers, id)
		}
	}
}

func (w *wireGuard) indexPeerDirectoryLocked(p *defs.WireGuardPeerConfig) {
	if p == nil || p.WireGuardPeerConfig == nil {
		return
	}
	// 1) peer.id
	if id := p.GetId(); id != 0 {
		w.peerDirectory[id] = proto.Clone(p.WireGuardPeerConfig).(*pb.WireGuardPeerConfig)
	}
	// 2) endpoint.wireguard_id锛堥儴鍒嗗満鏅?peer.id 鍙兘鏈～锛屼絾 endpoint 甯?wireguard_id锛?
	if p.GetEndpoint() != nil {
		if id := p.GetEndpoint().GetWireguardId(); id != 0 {
			w.peerDirectory[id] = proto.Clone(p.WireGuardPeerConfig).(*pb.WireGuardPeerConfig)
		}
	}
}

func (w *wireGuard) deletePeerDirectoryLocked(p *defs.WireGuardPeerConfig) {
	if p == nil {
		return
	}
	if id := p.GetId(); id != 0 {
		delete(w.peerDirectory, id)
	}
	if p.GetEndpoint() != nil {
		if id := p.GetEndpoint().GetWireguardId(); id != 0 {
			delete(w.peerDirectory, id)
		}
	}
}

// ensureConnectablePeersLocked 淇濊瘉鏈妭鐐瑰湪 wg 璁惧閲屽凡閰嶇疆鎵€鏈夆€滃綋鍓嶅彲鐩磋繛/鍙繛鎺モ€濈殑 peer锛?
// 浣嗚繖浜涜ˉ榻?peer 鐨?AllowedIPs 涓虹┖锛堝彧淇濇寔杩炴帴锛屼笉鎵胯浇璺敱锛夈€?
//
// 绾︽潫锛氬繀椤诲湪鎸佹湁 w.Lock() 鐨勬儏鍐典笅璋冪敤銆?
func (w *wireGuard) ensureConnectablePeersLocked() error {
	if w == nil || w.ifce == nil || w.wgDevice == nil {
		return nil
	}
	localID := w.ifce.GetId()
	if localID == 0 {
		return nil
	}
	adjs := w.ifce.GetAdjs()
	if adjs == nil {
		return nil
	}
	localLinks, ok := adjs[localID]
	if !ok || localLinks == nil || len(localLinks.GetLinks()) == 0 {
		return nil
	}

	// 褰撳墠鍙洿杩?鍙繛鎺ョ殑 peer id 闆嗗悎锛堟潵鑷?adj锛?
	connectable := make(map[uint32]struct{}, len(localLinks.GetLinks()))
	for _, l := range localLinks.GetLinks() {
		if l == nil {
			continue
		}
		toID := l.GetToWireguardId()
		if toID == 0 || toID == localID {
			continue
		}
		connectable[toID] = struct{}{}
	}

	log := w.svcLogger.WithField("op", "ensureConnectablePeers")
	log.Debugf("ensure connectable peers: local=%d connectable=%d peers=%d preconnect=%d directory=%d",
		localID, len(connectable), len(w.ifce.Peers), len(w.preconnectPeers), len(w.peerDirectory))

	// 褰撳墠宸查厤缃殑 peer锛氱敤 peer.id 涓?endpoint.wireguard_id 鍙岀储寮曪紝閬垮厤 peer.id 缂哄け瀵艰嚧閲嶅琛ラ綈
	exists := make(map[uint32]struct{}, len(w.ifce.Peers))
	for _, p := range w.ifce.GetParsedPeers() {
		if p == nil {
			continue
		}
		if id := p.GetId(); id != 0 {
			exists[id] = struct{}{}
		}
		if p.GetEndpoint() != nil {
			if id := p.GetEndpoint().GetWireguardId(); id != 0 {
				exists[id] = struct{}{}
			}
		}
	}

	uapiBuilder := NewUAPIBuilder()
	added := 0
	removed := 0
	skippedNoBase := 0
	skippedAlready := 0

	// 鍏堟竻鐞嗭細鐩告瘮涓婃锛屾湰娆℃嫇鎵戜腑宸测€滃畬鍏ㄤ笉鍙洿杩炩€濈殑 peer锛岄渶瑕佸交搴曚粠璁惧绉婚櫎
	// 浠呮竻鐞嗏€淎llowedIPs 涓虹┖鈥濈殑 peer锛堜篃灏辨槸涓嶆壙杞借矾鐢便€佸彧涓轰繚鎸佽繛鎺ヨ€屽瓨鍦ㄧ殑 peer锛?
	newPeers := make([]*pb.WireGuardPeerConfig, 0, len(w.ifce.Peers))
	for _, raw := range w.ifce.GetParsedPeers() {
		if raw == nil || raw.WireGuardPeerConfig == nil {
			continue
		}
		// 浠呭 AllowedIPs 涓虹┖鐨?peer 鍋氳嚜鍔ㄦ竻鐞?
		if len(raw.GetAllowedIps()) != 0 {
			newPeers = append(newPeers, raw.WireGuardPeerConfig)
			continue
		}

		var peerID uint32
		if raw.GetId() != 0 {
			peerID = raw.GetId()
		} else if raw.GetEndpoint() != nil && raw.GetEndpoint().GetWireguardId() != 0 {
			peerID = raw.GetEndpoint().GetWireguardId()
		}

		// 鏃犳硶璇嗗埆 peer id锛氫繚瀹堣捣瑙佷笉娓呯悊
		if peerID == 0 {
			newPeers = append(newPeers, raw.WireGuardPeerConfig)
			continue
		}

		// 褰撳墠鎷撴墤涓嶅彲鐩磋繛锛氬交搴曠Щ闄?
		if _, ok := connectable[peerID]; !ok {
			log.Debugf("preconnect remove: peerID=%d pk=%s (reason=not_connectable)", peerID, truncate(raw.GetPublicKey(), 10))
			uapiBuilder.RemovePeerByKey(raw.GetParsedPublicKey())
			delete(w.preconnectPeers, peerID)
			delete(exists, peerID)
			removed++
			continue
		}

		newPeers = append(newPeers, raw.WireGuardPeerConfig)
	}
	// 濡傛灉鏈夋竻鐞嗗彂鐢燂紝鍏堟洿鏂版湰鍦扮紦瀛橈紙璁惧鏇存柊鍦ㄦ渶鍚庣粺涓€ IpcSet锛?
	if removed > 0 {
		w.ifce.Peers = newPeers
	}

	for _, l := range localLinks.GetLinks() {
		if l == nil {
			continue
		}
		toID := l.GetToWireguardId()
		if toID == 0 || toID == localID {
			continue
		}
		if _, ok := exists[toID]; ok {
			skippedAlready++
			continue
		}

		base, ok := w.peerDirectory[toID]
		if !ok || base == nil || base.GetPublicKey() == "" {
			skippedNoBase++
			continue
		}
		cloned := &defs.WireGuardPeerConfig{WireGuardPeerConfig: proto.Clone(base).(*pb.WireGuardPeerConfig)}
		cloned.AllowedIps = nil
		if l.GetToEndpoint() != nil {
			cloned.Endpoint = l.GetToEndpoint()
		}
		if _, err := parseAndValidatePeerConfig(cloned); err != nil {
			continue
		}

		log.Debugf("preconnect add: peerID=%d pk=%s endpoint=%s",
			toID, truncate(cloned.GetPublicKey(), 10), endpointForLog(cloned.GetEndpoint()))
		uapiBuilder.AddPeerConfig(cloned)
		w.ifce.Peers = append(w.ifce.Peers, cloned.WireGuardPeerConfig)
		w.indexPeerDirectoryLocked(cloned)
		exists[toID] = struct{}{}
		w.preconnectPeers[toID] = struct{}{}
		added++
	}

	if added == 0 && removed == 0 {
		log.Debugf("ensure result: no-op (skippedAlready=%d skippedNoBase=%d)", skippedAlready, skippedNoBase)
		return nil
	}
	log.Debugf("ensure result: add=%d remove=%d skippedAlready=%d skippedNoBase=%d", added, removed, skippedAlready, skippedNoBase)
	if err := w.wgDevice.IpcSet(uapiBuilder.Build()); err != nil {
		log.WithError(err).Debugf("ensure IpcSet failed (add=%d remove=%d)", added, removed)
		return err
	}
	return nil
}

// mergeConnectablePeersFromAdj 灏嗘湰鑺傜偣 adj 鍥句腑鍙洿杩炵殑 peer 鍚堝苟杩涚洰鏍?peers銆?
//
// - **鍙湪鐩爣 peers 涓己澶辨椂鎵嶈ˉ榻?*锛堟寜 PublicKey 鍘婚噸锛夛紝閬垮厤瑕嗙洊鐢辫矾鐢辫鍒掑櫒璁＄畻鍑虹殑 AllowedIPs銆?
// - **琛ラ綈鐨?peer AllowedIPs 缃┖**锛岀‘淇濅笉浼氬紩鍏ラ澶栬矾鐢便€?
// - 濡傞摼璺樉寮忔惡甯?to_endpoint锛屽垯浼樺厛鐢ㄥ畠瑕嗙洊 peer.endpoint锛堢敤浜庡揩閫熸仮澶嶇洿杩烇級銆?
//
// knownPeers 鐢ㄤ簬鍦ㄧ洰鏍?peers 缂哄け鏃舵彁渚涒€滃彲鐢ㄧ殑 peer 鍩虹淇℃伅鈥濓紙鍏挜/棰勫叡浜瘑閽?绔偣绛夛級锛岄€氬父浼?oldPeers銆?
func mergeConnectablePeersFromAdj(ifce *defs.WireGuardConfig, desiredPeers []*defs.WireGuardPeerConfig, knownPeers []*defs.WireGuardPeerConfig) []*defs.WireGuardPeerConfig {
	if ifce == nil {
		return desiredPeers
	}
	localID := ifce.GetId()
	if localID == 0 {
		return desiredPeers
	}
	adjs := ifce.GetAdjs()
	if adjs == nil {
		return desiredPeers
	}
	localLinks, ok := adjs[localID]
	if !ok || localLinks == nil || len(localLinks.GetLinks()) == 0 {
		return desiredPeers
	}

	desiredByPK := make(map[string]*defs.WireGuardPeerConfig, len(desiredPeers))
	for _, p := range desiredPeers {
		if p == nil || p.GetPublicKey() == "" {
			continue
		}
		desiredByPK[p.GetPublicKey()] = p
	}

	// build id -> peer 鍩虹淇℃伅绱㈠紩锛堜紭鍏?desired锛屽叾娆?known锛?
	idToPeer := make(map[uint32]*defs.WireGuardPeerConfig, len(desiredPeers)+len(knownPeers))
	putPeerIDs := func(p *defs.WireGuardPeerConfig) {
		if p == nil {
			return
		}
		// 1) peer.id
		if id := p.GetId(); id != 0 {
			if _, exists := idToPeer[id]; !exists {
				idToPeer[id] = p
			}
		}
		// 2) endpoint.wireguard_id锛堟湁浜涗笅鍙戝満鏅彲鑳戒笉濉?peer.id锛屼絾 endpoint 閲屽甫 wireguard_id锛?
		if p.GetEndpoint() != nil {
			if id := p.GetEndpoint().GetWireguardId(); id != 0 {
				if _, exists := idToPeer[id]; !exists {
					idToPeer[id] = p
				}
			}
		}
	}
	for _, p := range desiredPeers {
		putPeerIDs(p)
	}
	for _, p := range knownPeers {
		putPeerIDs(p)
	}

	// 浠呰ˉ榻愶細adj 涓殑鐩磋繛鑺傜偣锛坱o_wireguard_id锛?
	for _, l := range localLinks.GetLinks() {
		if l == nil {
			continue
		}
		toID := l.GetToWireguardId()
		if toID == 0 || toID == localID {
			continue
		}

		base, ok := idToPeer[toID]
		if !ok || base == nil || base.GetPublicKey() == "" {
			continue
		}
		if _, exists := desiredByPK[base.GetPublicKey()]; exists {
			// 宸插湪鐩爣鍒楄〃涓紙閫氬父鍚湁璺敱瑙勫垝鍣ㄨ绠楀嚭鐨?AllowedIPs锛夛紝涓嶈鐩栥€?
			continue
		}

		// 澶嶅埗涓€浠斤紙閬垮厤鐩存帴鏀?oldPeers / knownPeers 鐨勫簳灞?pb 鎸囬拡锛?
		cloned := clonePeerConfig(base)
		// 涓嶅垎閰嶈矾鐢憋細AllowedIPs 缃┖
		cloned.AllowedIps = nil
		// 鏄惧紡閾捐矾 endpoint 浼樺厛
		if l.GetToEndpoint() != nil {
			cloned.Endpoint = l.GetToEndpoint()
		}
		// 纭繚 keepalive/AllowedIPs 鏍煎紡涓€鑷达紙澶嶇敤鐜版湁鏍￠獙閫昏緫锛?
		if _, err := parseAndValidatePeerConfig(cloned); err != nil {
			continue
		}

		desiredPeers = append(desiredPeers, cloned)
		desiredByPK[cloned.GetPublicKey()] = cloned
	}

	return desiredPeers
}

func clonePeerConfig(p *defs.WireGuardPeerConfig) *defs.WireGuardPeerConfig {
	if p == nil || p.WireGuardPeerConfig == nil {
		return &defs.WireGuardPeerConfig{}
	}

	// 浣跨敤 proto.Clone 閬垮厤鐩存帴鎷疯礉 protoimpl.MessageState锛堝唴閮ㄥ惈 mutex锛屼細瑙﹀彂鎷疯礉閿佸€肩殑鍛婅锛?
	cp, _ := proto.Clone(p.WireGuardPeerConfig).(*pb.WireGuardPeerConfig)
	if cp == nil {
		return &defs.WireGuardPeerConfig{}
	}
	return &defs.WireGuardPeerConfig{WireGuardPeerConfig: cp}
}

func endpointForLog(ep *pb.Endpoint) string {
	if ep == nil {
		return ""
	}
	if ep.GetUri() != "" {
		return ep.GetUri()
	}
	if ep.GetHost() != "" || ep.GetPort() != 0 {
		return fmt.Sprintf("%s:%d", ep.GetHost(), ep.GetPort())
	}
	return ""
}
