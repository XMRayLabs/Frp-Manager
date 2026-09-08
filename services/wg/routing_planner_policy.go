package wg

import (
	"math"
	"time"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
)

// RoutingPolicy 鍐冲畾杈规潈閲嶇殑璁＄畻鏂瑰紡銆?
// cost = LatencyTerm + InverseBandwidthTerm + HopWeight + HandshakePenalty
type RoutingPolicy struct {
	LatencyWeight          float64
	InverseBandwidthWeight float64
	HopWeight              float64
	MinUpMbps              uint32

	// LatencyBucketMs 鐢ㄤ簬瀵?latency 鍋氣€滃垎妗?閲忓寲鈥濓紝闄嶄綆鎶栧姩瀵艰嚧鐨勬渶鐭矾棰戠箒鍒囨崲銆?
	// 渚嬪 bucket=5ms锛屽垯 31/32/33ms 閮戒細琚噺鍖栦负 30/35ms 闄勮繎鐨勫悓涓€妗ｃ€?
	LatencyBucketMs uint32
	// MinLatencyMs/MaxLatencyMs 鐢ㄤ簬瀵?latency 鍋氶檺骞咃紝閬垮厤寮傚父鍊煎鏈€鐭矾浜х敓杩囧己鎵板姩銆?
	MinLatencyMs uint32
	MaxLatencyMs uint32
	// LatencyLogScale>0 鏃讹紝瀵?latency 浣跨敤 log1p 鍙樻崲骞朵箻浠ヨ绯绘暟锛屼娇鏉冮噸瀵瑰皬骞呮姈鍔ㄦ洿涓嶆晱鎰熴€?
	// 鑻ヤ负 0锛屽垯鍥為€€涓虹嚎鎬?latency銆?
	LatencyLogScale float64

	DefaultEndpointUpMbps    uint32
	DefaultEndpointLatencyMs uint32
	OfflineThreshold         time.Duration
	// HandshakeStaleThreshold/HandshakeStalePenalty 鐢ㄤ簬鎶戝埗鈥滄彙鎵嬭繃鏃р€濈殑閾捐矾琚€変负鏈€鐭矾銆?
	// 浠呭湪鑳戒粠 runtimeInfo 涓壘鍒板搴?peer 鐨?last_handshake_time_sec 鏃剁敓鏁堬紱鍚﹀垯涓嶆儵缃氾紙閬垮厤璇激锛夈€?
	HandshakeStaleThreshold time.Duration
	HandshakeStalePenalty   float64

	ACL                  *ACL
	NetworkTopologyCache app.NetworkTopologyCache
	CliMgr               app.ClientsManager
}

func (p *RoutingPolicy) LoadACL(acl *ACL) *RoutingPolicy {
	p.ACL = acl
	return p
}

func DefaultRoutingPolicy(acl *ACL, networkTopologyCache app.NetworkTopologyCache, cliMgr app.ClientsManager) RoutingPolicy {
	return RoutingPolicy{
		LatencyWeight:            1.0,
		InverseBandwidthWeight:   50.0, // 瀵逛綆甯﹀璺緞缁欎簣鏇撮珮鎯╃綒
		HopWeight:                1.0,
		MinUpMbps:                1,
		LatencyBucketMs:          5,
		MinLatencyMs:             1,
		MaxLatencyMs:             1500,
		LatencyLogScale:          10.0,
		DefaultEndpointUpMbps:    50,
		DefaultEndpointLatencyMs: 30,
		OfflineThreshold:         2 * time.Minute,
		// 榛樿鍚敤涓€涓俯鍜岀殑鈥滄彙鎵嬭繃鏃ф儵缃氣€濓細浼樺厛閫夋嫨杩戞湡鏈夋彙鎵嬬殑閾捐矾锛屼絾涓嶈嚦浜庡己鍒跺墧闄よ矾寰勩€?
		HandshakeStaleThreshold: 5 * time.Minute,
		HandshakeStalePenalty:   30.0,
		ACL:                     acl,
		NetworkTopologyCache:    networkTopologyCache,
		CliMgr:                  cliMgr,
	}
}

// EdgeWeight 璁＄畻涓€鏉♀€滄湁鍚戣竟鈥濈殑鏉冮噸锛堣秺灏忚秺浼橈級銆?
// 涓轰簡鎶戝埗寤惰繜鎺㈡祴鐨勫櫔澹板鑷磋矾鐢遍绻佹姈鍔紝杩欓噷瀵?latency 鍋氫簡锛氶檺骞?+ 鍒嗘《锛堝彲閫夛級+ log1p锛堝彲閫夛級銆?
func (p *RoutingPolicy) EdgeWeight(fromWGID uint, e Edge, idToPeer map[uint]*models.WireGuard) float64 {
	lat := float64(e.latency)

	// 1) 寤惰繜闄愬箙
	minLat := float64(p.MinLatencyMs)
	maxLat := float64(p.MaxLatencyMs)
	if minLat <= 0 {
		minLat = 1
	}
	if maxLat <= 0 {
		maxLat = 1500
	}
	if lat < minLat {
		lat = minLat
	}
	if lat > maxLat {
		lat = maxLat
	}

	// 2) 寤惰繜鍒嗘《锛堥噺鍖栵級
	if p.LatencyBucketMs > 0 {
		b := float64(p.LatencyBucketMs)
		// 鍥涜垗浜斿叆鍒版渶杩戞《
		lat = math.Floor((lat+b/2)/b) * b
		if lat < minLat {
			lat = minLat
		}
		if lat > maxLat {
			lat = maxLat
		}
	}

	// 3) 寤惰繜椤癸細log1p锛堝彲閫夛級+ scale
	latencyTerm := 0.0
	if p.LatencyWeight != 0 {
		if p.LatencyLogScale > 0 {
			latencyTerm = p.LatencyWeight * math.Log1p(lat) * p.LatencyLogScale
		} else {
			latencyTerm = p.LatencyWeight * lat
		}
	}

	// 4) 甯﹀椤癸細瀵逛綆甯﹀鏇存晱鎰燂紱浣跨敤 MinUpMbps 鍋氫笅闄愰伩鍏嶆瀬绔€?
	minUp := float64(p.MinUpMbps)
	if minUp <= 0 {
		minUp = 1
	}
	up := math.Max(float64(e.upMbps), minUp)
	invBw := 1.0 / math.Max(up, 1e-6)
	bwTerm := p.InverseBandwidthWeight * invBw

	// 5) hop 椤?
	hopTerm := p.HopWeight

	// 6) 鎻℃墜杩囨棫鎯╃綒锛氬繀椤绘棤鏂瑰悜
	handshakePenalty := 0.0
	if p.HandshakeStalePenalty > 0 && p.HandshakeStaleThreshold > 0 {
		if age, ok := getHandshakeAgeBetween(fromWGID, e.to, idToPeer, *p); ok && age > p.HandshakeStaleThreshold {
			handshakePenalty = p.HandshakeStalePenalty
		}
	}

	return latencyTerm + bwTerm + hopTerm + handshakePenalty
}
