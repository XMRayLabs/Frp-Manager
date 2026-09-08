package wg

import (
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/wg"
	"github.com/samber/lo"
)

func adjsToPB(resp map[uint][]wg.Edge) map[uint32]*pb.WireGuardLinks {
	adjs := make(map[uint32]*pb.WireGuardLinks)
	for id, peerConfigs := range resp {
		adjs[uint32(id)] = &pb.WireGuardLinks{
			Links: lo.Map(peerConfigs, func(e wg.Edge, _ int) *pb.WireGuardLink {
				v := e.ToPB()
				v.FromWireguardId = uint32(uint(id))
				return v
			}),
		}
	}

	for id, links := range adjs {
		for _, link := range links.GetLinks() {
			toWireguardEdges, ok := adjs[uint32(link.GetToWireguardId())]
			if ok {
				for _, edge := range toWireguardEdges.GetLinks() {
					if edge.GetToWireguardId() == uint32(uint(id)) {
						link.DownBandwidthMbps = edge.GetUpBandwidthMbps()
					}
				}
			}
		}
	}

	return adjs
}

// peerConfigsToPBAdjs 灏嗏€滅湡瀹炰笅鍙戠粰鑺傜偣鐨勮矾鐢辫〃锛圥eerConfig.AllowedIps锛夆€濊浆鎹负鎷撴墤灞曠ず鎵€闇€鐨?Adjs銆?
//
// - routes: 鐩存帴浣跨敤 peerCfg.AllowedIps锛堣繖鎵嶆槸 WireGuard 瀹為檯浣跨敤鐨勮矾鐢辫〃锛?
// - latency/up/down: 灏介噺浠?allEdges锛坆uildAdjacency 鐨勭洿杩炶竟鎸囨爣锛変腑琛ラ綈锛屼粎鐢ㄤ簬灞曠ず
// - endpoint: 浼樺厛浣跨敤 peerCfg.Endpoint锛堜笌瀹為檯涓嬪彂涓€鑷达級
func peerConfigsToPBAdjs(peerCfgs map[uint][]*pb.WireGuardPeerConfig, allEdges map[uint][]wg.Edge) map[uint32]*pb.WireGuardLinks {
	adjs := make(map[uint32]*pb.WireGuardLinks, len(peerCfgs))

	for src, pcs := range peerCfgs {
		srcID := uint32(src)
		links := make([]*pb.WireGuardLink, 0, len(pcs))

		// 鏋勫缓 toID -> edge 鎸囨爣绱㈠紩锛堜粎鐢ㄤ簬灞曠ず latency/up锛?
		edgePBByTo := make(map[uint32]*pb.WireGuardLink, 16)
		for _, e := range allEdges[src] {
			epb := e.ToPB()
			edgePBByTo[epb.GetToWireguardId()] = epb
		}

		for _, pc := range pcs {
			if pc == nil || pc.GetId() == 0 {
				continue
			}
			toID := pc.GetId()
			var latency uint32
			var up uint32
			if epb, ok := edgePBByTo[toID]; ok && epb != nil {
				latency = epb.GetLatencyMs()
				up = epb.GetUpBandwidthMbps()
			}

			links = append(links, &pb.WireGuardLink{
				FromWireguardId:   srcID,
				ToWireguardId:     toID,
				LatencyMs:         latency,
				UpBandwidthMbps:   up,
				DownBandwidthMbps: 0, // 涓嬮潰缁熶竴濉厖
				Active:            true,
				ToEndpoint:        pc.GetEndpoint(),
				Routes:            pc.GetAllowedIps(),
			})
		}

		adjs[srcID] = &pb.WireGuardLinks{Links: links}
	}

	// 濉厖 down bandwidth锛堝弬鑰?adjsToPB 鐨勫仛娉曪細鍙栧弽鍚戣竟鐨?up锛?
	for id, links := range adjs {
		for _, link := range links.GetLinks() {
			toWireguardEdges, ok := adjs[uint32(link.GetToWireguardId())]
			if ok {
				for _, edge := range toWireguardEdges.GetLinks() {
					if edge.GetToWireguardId() == uint32(uint(id)) {
						link.DownBandwidthMbps = edge.GetUpBandwidthMbps()
					}
				}
			}
		}
	}

	return adjs
}
