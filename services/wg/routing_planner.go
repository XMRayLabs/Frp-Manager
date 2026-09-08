package wg

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sort"
	"time"

	"github.com/samber/lo"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
)

// WireGuard 鐨?AllowedIPs 鍚屾椂鎵挎媴涓や欢浜嬶細
// 1) 鍑虹珯閫夎矾锛氱洰鐨?IP 鍖归厤鍝釜 peer 鐨?AllowedIPs锛屽氨鎶婂寘鍙戠粰鍝釜 peer
// 2) 鍏ョ珯婧愬湴鍧€鏍￠獙锛氫粠鏌?peer 瑙ｅ瘑鍑烘潵鐨?inner packet锛屽叾 source IP 蹇呴』钀藉湪璇?peer 鐨?AllowedIPs
//
// 鍥犳锛屽璺宠浆鍙戞椂锛屾煇鑺傜偣 i 浠庘€滀笂涓€璺?peer=j鈥濇敹鍒扮殑鍖咃紝鍏?inner source 浠嶆槸鈥滃師濮嬫簮鑺傜偣 s 鐨?/32鈥濓紝
// 鎵€浠?i 閰嶇疆 peer(j) 鐨?AllowedIPs 蹇呴』鍖呭惈杩欎簺浼氱粡鐢?j 杞彂杩涙潵鐨勨€滄簮鍦板潃闆嗗悎鈥濓紝鍚﹀垯浼氱洿鎺ヤ涪鍖呫€?
// 鎬濊矾锛?
// - 鍦ㄤ竴涓€滃绉版潈閲嶁€濈殑鍥句笂鍋氭渶鐭矾锛堜繚璇佽矾寰勫彲閫嗭紝閬垮厤閲嶅/鍐茬獊锛?
// - 鍚屾椂浜у嚭锛?
//   - Out(i->nextHop): i 鍑虹珯鏃讹紝鍝簺鐩殑 /32 搴旇蛋 nextHop锛堢洰鐨勯泦鍚堬級
//   - In(i<-prevHop): i 鍏ョ珯鏃讹紝浠?prevHop 杩囨潵鐨勫寘鍏佽鍝簺婧?/32锛堟簮闆嗗悎锛?
// - 鏈€缁堝姣忎釜 i 鐨勬瘡涓洿杩?peer(j)锛孉llowedIPs = Out(i->j) 鈭?In(i<-j)
// - 涓ユ牸鏍￠獙锛氬鍚屼竴鑺傜偣 i锛屼笉鍏佽鍑虹幇鍚屼竴涓?/32 鍚屾椂鍑虹幇鍦ㄥ涓?peer 鐨?AllowedIPs锛堝惁鍒?WG 琛屼负涓嶇‘瀹氾級

type AllowedIPsPlanner interface {
	// Compute 鍩轰簬鎷撴墤涓庨摼璺寚鏍囷紝璁＄畻姣忎釜鑺傜偣搴旈厤缃埌鐩磋繛閭诲眳鐨?AllowedIPs銆?
	// 杈撳叆鐨?peers 搴斿寘鍚悓涓€ Network 涓嬬殑鎵€鏈?WireGuard 鑺傜偣锛宭inks 涓哄叾鏈夊悜閾捐矾銆?
	// 杩斿洖锛氳妭鐐笽D->PeerConfig 鍒楄〃锛岃妭鐐笽D->Edge 鍒楄〃锛堝畬鏁村€欓€夊浘锛岀敤浜庡睍绀猴級銆?
	Compute(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]*pb.WireGuardPeerConfig, map[uint][]Edge, error)
	// BuildGraph 鍩轰簬鎷撴墤涓庨摼璺寚鏍囷紝杩斿洖瀹屾暣鍊欓€夊浘锛堢敤浜庡睍绀?璇婃柇锛夈€?
	BuildGraph(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]Edge, error)
	// BuildFinalGraph 杩斿洖鈥滄渶缁堜笅鍙戠殑鐩磋繛杈光€濅笌鍏?routes锛堢敤浜庡睍绀?SPF 缁撴灉锛夈€?
	BuildFinalGraph(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]Edge, error)
}

type dijkstraAllowedIPsPlanner struct {
	policy RoutingPolicy
}

func NewDijkstraAllowedIPsPlanner(policy RoutingPolicy) AllowedIPsPlanner {
	return &dijkstraAllowedIPsPlanner{policy: policy}
}

func PlanAllowedIPs(peers []*models.WireGuard, links []*models.WireGuardLink, policy RoutingPolicy) (map[uint][]*pb.WireGuardPeerConfig, map[uint][]Edge, error) {
	return NewDijkstraAllowedIPsPlanner(policy).Compute(peers, links)
}

func (p *dijkstraAllowedIPsPlanner) Compute(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]*pb.WireGuardPeerConfig, map[uint][]Edge, error) {
	if len(peers) == 0 {
		return map[uint][]*pb.WireGuardPeerConfig{}, map[uint][]Edge{}, nil
	}

	idToPeer, order := buildNodeIndexSorted(peers)
	cidrByID, err := buildNodeCIDRMap(order, idToPeer)
	if err != nil {
		return nil, nil, err
	}

	adj := buildAdjacency(order, idToPeer, links, p.policy)
	// SPF 鍙備笌鐨勮竟锛氭樉寮忚竟 + 宸叉帰娴嬪彲杈剧殑鎺ㄦ柇杈癸紝骞惰姹傗€滃彲鐢ㄤ簬杞彂鈥濈殑杈瑰繀椤诲弻鍚戝瓨鍦?
	spfAdj := filterAdjacencyForSPF(order, adj, p.policy)
	spfAdj = filterAdjacencyForSymmetricLinks(order, spfAdj)

	peerCfgs, finalEdges, err := computeAllowedIPs(order, idToPeer, cidrByID, spfAdj, adj, p.policy)
	if err != nil {
		return nil, nil, err
	}

	// 濉厖娌℃湁閾捐矾鐨勮妭鐐癸紙灞曠ず鐢級
	for _, id := range order {
		if _, ok := adj[id]; !ok {
			adj[id] = []Edge{}
		}
		if _, ok := finalEdges[id]; !ok {
			finalEdges[id] = []Edge{}
		}
		if _, ok := peerCfgs[id]; !ok {
			peerCfgs[id] = []*pb.WireGuardPeerConfig{}
		}
	}

	return peerCfgs, adj, nil
}

func (p *dijkstraAllowedIPsPlanner) BuildGraph(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]Edge, error) {
	if len(peers) == 0 {
		return map[uint][]Edge{}, nil
	}
	idToPeer, order := buildNodeIndexSorted(peers)
	adj := buildAdjacency(order, idToPeer, links, p.policy)
	for _, id := range order {
		if _, ok := adj[id]; !ok {
			adj[id] = []Edge{}
		}
	}
	return adj, nil
}

func (p *dijkstraAllowedIPsPlanner) BuildFinalGraph(peers []*models.WireGuard, links []*models.WireGuardLink) (map[uint][]Edge, error) {
	if len(peers) == 0 {
		return map[uint][]Edge{}, nil
	}

	idToPeer, order := buildNodeIndexSorted(peers)
	cidrByID, err := buildNodeCIDRMap(order, idToPeer)
	if err != nil {
		return nil, err
	}

	adj := buildAdjacency(order, idToPeer, links, p.policy)
	spfAdj := filterAdjacencyForSPF(order, adj, p.policy)
	spfAdj = filterAdjacencyForSymmetricLinks(order, spfAdj)

	_, finalEdges, err := computeAllowedIPs(order, idToPeer, cidrByID, spfAdj, adj, p.policy)
	if err != nil {
		return nil, err
	}
	for _, id := range order {
		if _, ok := finalEdges[id]; !ok {
			finalEdges[id] = []Edge{}
		}
	}
	return finalEdges, nil
}

// Edge 琛ㄧず鍊欓€?鏈€缁堝浘閲岀殑鈥滄湁鍚戠洿杩炶竟鈥濄€?
type Edge struct {
	to         uint
	latency    uint32
	upMbps     uint32
	toEndpoint *models.Endpoint // 鎸囧畾鐨勭洰鏍囩鐐癸紝鍙兘涓?nil
	routes     []string         // 鏈€缁堝睍绀猴細璇ョ洿杩?peer 鎵胯浇鐨勮矾鐢憋紙AllowedIPs锛?
	explicit   bool             // true: 鏄惧紡 link锛沠alse: 鎺ㄦ柇/鎺㈡祴鐢?link
}

func (e *Edge) ToPB() *pb.WireGuardLink {
	link := &pb.WireGuardLink{
		ToWireguardId:   uint32(e.to),
		LatencyMs:       e.latency,
		UpBandwidthMbps: e.upMbps,
		Active:          true,
		Routes:          e.routes,
	}
	if e.toEndpoint != nil {
		link.ToEndpoint = e.toEndpoint.ToPB()
	}
	return link
}

// buildNodeIndexSorted 杩斿洖锛歩d->peer 鏄犲皠 涓?鎸?id 鎺掑簭鐨?order锛堢敤浜庣‘瀹氭€э級
func buildNodeIndexSorted(peers []*models.WireGuard) (map[uint]*models.WireGuard, []uint) {
	idToPeer := make(map[uint]*models.WireGuard, len(peers))
	order := make([]uint, 0, len(peers))
	for _, p := range peers {
		if p == nil {
			continue
		}
		id := uint(p.ID)
		idToPeer[id] = p
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	return idToPeer, order
}

func buildNodeCIDRMap(order []uint, idToPeer map[uint]*models.WireGuard) (map[uint]string, error) {
	out := make(map[uint]string, len(order))
	for _, id := range order {
		p := idToPeer[id]
		if p == nil {
			continue
		}
		base, err := p.AsBasePeerConfig(nil)
		if err != nil || len(base.GetAllowedIps()) == 0 {
			return nil, fmt.Errorf("invalid wireguard local address for id=%d", id)
		}
		out[id] = base.GetAllowedIps()[0]
	}
	return out, nil
}

// buildAdjacency 鏋勫缓鈥滃€欓€夌洿杩炶竟鈥濓細
// 1) 鏄惧紡閾捐矾锛堢鐞嗗憳閰嶇疆锛夌洿鎺ュ姞鍏?
// 2) 鑻ユ煇鑺傜偣鍏峰 endpoint锛屽垯鍏朵粬鑺傜偣鍙寜 ACL 鎺ㄦ柇鐩磋繛瀹冿紙鐢ㄤ簬鎺㈡祴/鍊欓€夛級
func buildAdjacency(order []uint, idToPeer map[uint]*models.WireGuard, links []*models.WireGuardLink, policy RoutingPolicy) map[uint][]Edge {
	adj := make(map[uint][]Edge, len(order))

	online := func(id uint) bool {
		if policy.CliMgr == nil {
			return true
		}
		p := idToPeer[id]
		if p == nil || p.ClientID == "" {
			return false
		}
		lastSeenAt, ok := policy.CliMgr.GetLastSeenAt(p.ClientID)
		if !ok {
			return false
		}
		if policy.OfflineThreshold > 0 && time.Since(lastSeenAt) > policy.OfflineThreshold {
			return false
		}
		return true
	}

	// 1) 鏄惧紡閾捐矾
	for _, l := range links {
		if l == nil || !l.Active {
			continue
		}
		from := l.FromWireGuardID
		to := l.ToWireGuardID
		if _, ok := idToPeer[from]; !ok {
			continue
		}
		if _, ok := idToPeer[to]; !ok {
			continue
		}
		if !online(from) || !online(to) {
			continue
		}
		// 濡傛灉涓や釜 peer 閮芥病鏈?endpoint锛屽垯涓嶅缓绔嬮摼璺紙鏃犳硶鐩磋繛锛?
		if len(idToPeer[from].AdvertisedEndpoints) == 0 && len(idToPeer[to].AdvertisedEndpoints) == 0 {
			continue
		}

		latency := l.LatencyMs
		if latency == 0 {
			if policy.NetworkTopologyCache != nil {
				if latencyMs, ok := policy.NetworkTopologyCache.GetLatencyMs(from, to); ok {
					latency = latencyMs
				}
			}
			if latency == 0 {
				latency = policy.DefaultEndpointLatencyMs
			}
		}

		adj[from] = append(adj[from], Edge{
			to:         to,
			latency:    latency,
			upMbps:     l.UpBandwidthMbps,
			toEndpoint: l.ToEndpoint,
			explicit:   true,
		})
	}

	// 2) 鎺ㄦ柇/鎺㈡祴鐢ㄨ竟锛氳嫢鏌愯妭鐐瑰叿澶?endpoint锛屽垯鎵€鏈夊叾浠栬妭鐐瑰彲鐩磋繛瀹?
	edgeSet := make(map[[2]uint]struct{}, 64)
	for from, edges := range adj {
		for _, e := range edges {
			edgeSet[[2]uint{from, e.to}] = struct{}{}
		}
	}

	for _, to := range order {
		peerTo := idToPeer[to]
		if peerTo == nil || len(peerTo.AdvertisedEndpoints) == 0 {
			continue
		}
		for _, from := range order {
			if from == to {
				continue
			}
			if _, ok := idToPeer[from]; !ok {
				continue
			}
			if !online(from) || !online(to) {
				continue
			}

			latency := policy.DefaultEndpointLatencyMs
			if policy.NetworkTopologyCache != nil {
				if latencyMs, ok := policy.NetworkTopologyCache.GetLatencyMs(from, to); ok {
					latency = latencyMs
				}
			}

			// 娉ㄦ剰锛氭帹鏂竟闇€瑕佹寜鈥滀袱涓柟鍚戔€濆垎鍒垽鏂?ACL 骞跺垎鍒缓杈广€?
			// 杩欐牱鍗充娇 from 娌℃湁 endpoint锛屼篃鑳借 endpoint 鑺傜偣绾冲叆閭绘帴锛堟弧瓒冲绉扮洿杩?peer 鐨勮姹傦級銆?

			// from -> to
			if policy.ACL == nil || policy.ACL.CanConnect(idToPeer[from], idToPeer[to]) {
				key := [2]uint{from, to}
				if _, exists := edgeSet[key]; !exists {
					adj[from] = append(adj[from], Edge{
						to:       to,
						latency:  latency,
						upMbps:   policy.DefaultEndpointUpMbps,
						explicit: false,
					})
					edgeSet[key] = struct{}{}
				}
			}

			// to -> from锛堝弽鍚戣竟鍚屾牱浣跨敤鍚屼竴瀵硅妭鐐圭殑 latency 浼拌锛汫etLatencyMs 鏈韩宸插仛姝ｅ弽鍚戝厹搴曪級
			if policy.ACL == nil || policy.ACL.CanConnect(idToPeer[to], idToPeer[from]) {
				key := [2]uint{to, from}
				if _, exists := edgeSet[key]; !exists {
					adj[to] = append(adj[to], Edge{
						to:       from,
						latency:  latency,
						upMbps:   policy.DefaultEndpointUpMbps,
						explicit: false,
					})
					edgeSet[key] = struct{}{}
				}
			}
		}
	}

	// 绋冲畾鎺掑簭锛氫繚璇侀亶鍘嗛『搴忕‘瀹氭€?
	for _, from := range order {
		if edges, ok := adj[from]; ok {
			sort.SliceStable(edges, func(i, j int) bool {
				if edges[i].explicit != edges[j].explicit {
					return edges[i].explicit // explicit 浼樺厛
				}
				return edges[i].to < edges[j].to
			})
			adj[from] = edges
		}
	}

	return adj
}

func isUnreachableLatency(latency uint32) bool {
	// 鍏煎涓ょ被涓嶅彲杈惧摠鍏碉細
	// - math.MaxUint32锛堝巻鍙插疄鐜帮級
	// - math.MaxInt32锛堥儴鍒嗗睍绀?杞崲閾捐矾閲屼細鍑虹幇 2147483647锛?
	return latency == math.MaxUint32 || latency == uint32(math.MaxInt32)
}

// filterAdjacencyForSPF锛氭樉寮忚竟鐩存帴淇濈暀锛涙帹鏂竟蹇呴』鏈夋帰娴嬫暟鎹紝涓斾笉鍙揪鍝ㄥ叺鍊煎墧闄?
func filterAdjacencyForSPF(order []uint, adj map[uint][]Edge, policy RoutingPolicy) map[uint][]Edge {
	ret := make(map[uint][]Edge, len(order))
	for from, edges := range adj {
		for _, e := range edges {
			if e.explicit {
				ret[from] = append(ret[from], e)
				continue
			}
			if policy.NetworkTopologyCache == nil {
				continue
			}
			latency, ok := policy.NetworkTopologyCache.GetLatencyMs(from, e.to)
			if !ok {
				continue
			}
			if isUnreachableLatency(latency) {
				continue
			}
			e.latency = latency
			ret[from] = append(ret[from], e)
		}
	}
	for _, id := range order {
		if _, ok := ret[id]; !ok {
			ret[id] = []Edge{}
		}
	}
	return ret
}

// filterAdjacencyForSymmetricLinks 浠呬繚鐣欌€滃瓨鍦ㄥ弽鍚戠洿杩炶竟鈥濈殑閭绘帴锛堢敤浜庡彲杞彂 SPF锛夈€?
func filterAdjacencyForSymmetricLinks(order []uint, adj map[uint][]Edge) map[uint][]Edge {
	ret := make(map[uint][]Edge, len(order))
	edgeSet := make(map[[2]uint]struct{}, 64)
	for from, edges := range adj {
		for _, e := range edges {
			edgeSet[[2]uint{from, e.to}] = struct{}{}
		}
	}
	for from, edges := range adj {
		for _, e := range edges {
			if _, ok := edgeSet[[2]uint{e.to, from}]; !ok {
				continue
			}
			ret[from] = append(ret[from], e)
		}
	}
	for _, id := range order {
		if _, ok := ret[id]; !ok {
			ret[id] = []Edge{}
		}
	}
	return ret
}

type directedEdgeInfo struct {
	latency    uint32
	upMbps     uint32
	toEndpoint *models.Endpoint
	explicit   bool
}

type undirectedNeighbor struct {
	to     uint
	weight float64
}

// computeAllowedIPs 鏄€滄渶缁堜笅鍙戣矾鐢扁€濈殑鏍稿績锛?
// - 鍦?spfAdj 涓婃瀯寤衡€滃绉版潈閲嶇殑鏃犲悜鍥锯€?
// - 瀵规瘡涓?src 鍋氫竴娆?Dijkstra锛屽緱鍒版渶鐭矾鏍?prev
// - 鍚屾椂鐢熸垚 Out(dst prefixes) 涓?In(src prefixes) 骞跺悎骞跺埌姣忔潯鐩磋繛 peer 鐨?AllowedIPs
func computeAllowedIPs(
	order []uint,
	idToPeer map[uint]*models.WireGuard,
	cidrByID map[uint]string,
	spfAdj map[uint][]Edge,
	fullAdj map[uint][]Edge, // 鐢ㄤ簬灞曠ず琛ラ綈 latency/up/endpoint
	policy RoutingPolicy,
) (map[uint][]*pb.WireGuardPeerConfig, map[uint][]Edge, error) {
	// 鏋勫缓 directed edge info锛堢敤浜?endpoint/灞曠ず锛夛紝骞舵瀯寤?undirected graph锛堝绉版潈閲嶏級
	dInfo := make(map[[2]uint]*directedEdgeInfo, 128)
	undir := make(map[uint][]undirectedNeighbor, len(order))

	// 鍏堟妸 spfAdj 鐨?directed info 璁颁笅鏉?
	for _, from := range order {
		for _, e := range spfAdj[from] {
			key := [2]uint{from, e.to}
			dInfo[key] = &directedEdgeInfo{
				latency:    e.latency,
				upMbps:     e.upMbps,
				toEndpoint: e.toEndpoint,
				explicit:   e.explicit,
			}
		}
	}

	// 鏃犲悜鍥撅細鍙坊鍔犫€滄垚瀵瑰瓨鍦ㄢ€濈殑杈癸紝weight 鐢?max(w_uv, w_vu) 淇濊瘉瀵圭О
	added := make(map[[2]uint]struct{}, 128)
	for _, u := range order {
		for _, e := range spfAdj[u] {
			v := e.to
			if u == v {
				continue
			}
			// 鍙鐞嗕竴娆?pair(u,v)
			a, b := u, v
			if a > b {
				a, b = b, a
			}
			pair := [2]uint{a, b}
			if _, ok := added[pair]; ok {
				continue
			}
			// 闇€瑕佸弻鍚戣竟淇℃伅
			uv, ok1 := dInfo[[2]uint{u, v}]
			vu, ok2 := dInfo[[2]uint{v, u}]
			if !ok1 || !ok2 || uv == nil || vu == nil {
				continue
			}
			// 鐢?policy.EdgeWeight 璁＄畻鍙屽悜鏉冮噸骞跺彇 max 鍋氬绉?
			wuv := policy.EdgeWeight(u, Edge{to: v, latency: uv.latency, upMbps: uv.upMbps, toEndpoint: uv.toEndpoint, explicit: uv.explicit}, idToPeer)
			wvu := policy.EdgeWeight(v, Edge{to: u, latency: vu.latency, upMbps: vu.upMbps, toEndpoint: vu.toEndpoint, explicit: vu.explicit}, idToPeer)
			w := math.Max(wuv, wvu)
			undir[a] = append(undir[a], undirectedNeighbor{to: b, weight: w})
			undir[b] = append(undir[b], undirectedNeighbor{to: a, weight: w})
			added[pair] = struct{}{}
		}
	}

	// 绋冲畾鎺掑簭
	for _, u := range order {
		neis := undir[u]
		sort.SliceStable(neis, func(i, j int) bool { return neis[i].to < neis[j].to })
		undir[u] = neis
	}

	// Out/ In 鑱氬悎锛歰wner -> peer -> set[cidr]
	allowed := make(map[uint]map[uint]map[string]struct{}, len(order))

	for _, src := range order {
		dist := make(map[uint]float64, len(order))
		prev := make(map[uint]uint, len(order)) // prev[dst] = predecessor of dst on path from src
		visited := make(map[uint]bool, len(order))
		for _, id := range order {
			dist[id] = math.Inf(1)
		}
		dist[src] = 0

		// Dijkstra锛圤(n^2)锛岃妭鐐规暟閫氬父涓嶅ぇ锛涘悓鏃朵繚璇佺‘瀹氭€э級
		for {
			u, ok := pickNext(order, dist, visited)
			if !ok {
				break
			}
			visited[u] = true
			for _, nb := range undir[u] {
				v := nb.to
				if visited[v] {
					continue
				}
				alt := dist[u] + nb.weight
				if alt < dist[v] {
					dist[v] = alt
					prev[v] = u
					continue
				}
				// tie-break锛氱浉鍚岃窛绂绘椂锛岄€夋嫨鏇村皬鐨?predecessor锛岀‘淇濈ǔ瀹?
				if alt == dist[v] {
					if cur, ok := prev[v]; !ok || u < cur {
						prev[v] = u
					}
				}
			}
		}

		// 1) 鍑虹珯鐩殑闆嗗悎锛歞stCIDR -> nextHop(src,dst)
		for _, dst := range order {
			if dst == src {
				continue
			}
			if _, ok := prev[dst]; !ok {
				continue // unreachable
			}
			next := findNextHop(src, dst, prev)
			if next == 0 {
				continue
			}
			cidr := cidrByID[dst]
			if cidr == "" {
				continue
			}
			ensureAllowedSet(allowed, src, next)[cidr] = struct{}{}
		}

		// 2) 鍏ョ珯婧愰泦鍚堬細srcCIDR -> prevHop(src,dst) 褰掑埌 dst 鑺傜偣鐨?peer(prevHop)
		srcCIDR := cidrByID[src]
		if srcCIDR != "" {
			for _, dst := range order {
				if dst == src {
					continue
				}
				pred, ok := prev[dst]
				if !ok || pred == 0 {
					continue
				}
				ensureAllowedSet(allowed, dst, pred)[srcCIDR] = struct{}{}
			}
		}
	}

	// 鏋勫缓 PeerConfigs锛屽苟鍋氬己鏍￠獙锛堝悓涓€鑺傜偣涓嶅厑璁?CIDR 鍒嗛厤鍒板涓?peer锛?
	result := make(map[uint][]*pb.WireGuardPeerConfig, len(order))
	finalEdges := make(map[uint][]Edge, len(order))

	for _, owner := range order {
		peerToCIDRs := allowed[owner]
		if len(peerToCIDRs) == 0 {
			result[owner] = []*pb.WireGuardPeerConfig{}
			finalEdges[owner] = []Edge{}
			continue
		}

		seen := make(map[string]uint, 128)
		peerIDs := lo.Keys(peerToCIDRs)
		sort.Slice(peerIDs, func(i, j int) bool { return peerIDs[i] < peerIDs[j] })

		pcs := make([]*pb.WireGuardPeerConfig, 0, len(peerIDs))
		edges := make([]Edge, 0, len(peerIDs))

		for _, peerID := range peerIDs {
			cset := peerToCIDRs[peerID]
			if len(cset) == 0 {
				continue
			}
			remote := idToPeer[peerID]
			if remote == nil {
				continue
			}

			// endpoint锛氫紭鍏堜娇鐢?spfAdj 鐨勭洿杩炶竟鐨?toEndpoint锛堜笌瀹為檯鏇翠竴鑷达級
			var specifiedEndpoint *models.Endpoint
			if info := dInfo[[2]uint{owner, peerID}]; info != nil && info.toEndpoint != nil {
				specifiedEndpoint = info.toEndpoint
			}

			base, err := remote.AsBasePeerConfig(specifiedEndpoint)
			if err != nil {
				return nil, nil, errors.Join(errors.New("build peer base config failed"), err)
			}

			cidrs := make([]string, 0, len(cset))
			for c := range cset {
				if prevOwner, ok := seen[c]; ok && prevOwner != peerID {
					return nil, nil, fmt.Errorf("duplicate allowed ip on node %d: %s appears in peer %d and peer %d", owner, c, prevOwner, peerID)
				}
				seen[c] = peerID
				cidrs = append(cidrs, c)
			}
			sort.Strings(cidrs)
			base.AllowedIps = lo.Uniq(cidrs)
			pcs = append(pcs, base)

			// 鐢?fullAdj 琛ラ綈灞曠ず鎸囨爣锛坙atency/up/endpoint锛?
			lat, up, ep, explicit := lookupEdgeForDisplay(fullAdj, owner, peerID)
			edges = append(edges, Edge{
				to:         peerID,
				latency:    lat,
				upMbps:     up,
				toEndpoint: ep,
				routes:     base.AllowedIps,
				explicit:   explicit,
			})
		}

		// 鎸?client_id 绋冲畾鎺掑簭锛堜繚鎸佸師鎺ュ彛涔犳儻锛?
		sort.SliceStable(pcs, func(i, j int) bool { return pcs[i].GetClientId() < pcs[j].GetClientId() })
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].to < edges[j].to })

		result[owner] = pcs
		finalEdges[owner] = edges
	}

	return result, finalEdges, nil
}

func ensureAllowedSet(m map[uint]map[uint]map[string]struct{}, owner, peer uint) map[string]struct{} {
	if _, ok := m[owner]; !ok {
		m[owner] = make(map[uint]map[string]struct{}, 8)
	}
	if _, ok := m[owner][peer]; !ok {
		m[owner][peer] = make(map[string]struct{}, 32)
	}
	return m[owner][peer]
}

func lookupEdgeForDisplay(fullAdj map[uint][]Edge, from, to uint) (latency uint32, up uint32, ep *models.Endpoint, explicit bool) {
	edges := fullAdj[from]
	for _, e := range edges {
		if e.to == to {
			return e.latency, e.upMbps, e.toEndpoint, e.explicit
		}
	}
	return 0, 0, nil, false
}

func pickNext(order []uint, dist map[uint]float64, visited map[uint]bool) (uint, bool) {
	best := uint(0)
	bestVal := math.Inf(1)
	found := false
	for _, vid := range order {
		if visited[vid] {
			continue
		}
		if dist[vid] < bestVal {
			bestVal = dist[vid]
			best = vid
			found = true
		}
	}
	return best, found
}

// findNextHop 杩斿洖浠?src 鍒?dst 鐨?nextHop锛坰rc 鐨勭洿杩為偦灞咃級锛屼緷璧?prev[dst] = predecessor(dst)
func findNextHop(src, dst uint, prev map[uint]uint) uint {
	next := dst
	for {
		p, ok := prev[next]
		if !ok {
			return 0
		}
		if p == src {
			return next
		}
		next = p
	}
}

// 浠呯敤浜庢祴璇?璇婃柇锛氳В鏋?/32 鐨?host ip锛堟牎楠屾牸寮忥級
func parseHostFromCIDR(c string) (netip.Addr, bool) {
	p, err := netip.ParsePrefix(c)
	if err != nil {
		return netip.Addr{}, false
	}
	return p.Addr(), true
}

// getHandshakeAgeBetween 杩斿洖 a<->b 闂?peer handshake 鐨勨€滄渶澶р€濆勾榫勶紙鍙浠绘剰涓€渚у彲瑙傛祴鍒版彙鎵嬫椂闂村氨鐢熸晥锛夈€?
// 閫夋嫨 max 鐨勫師鍥狅細濡傛灉浠讳竴鏂瑰悜鎻℃墜杩囨棫锛岄兘搴旀姂鍒惰繖瀵硅妭鐐逛綔涓哄彲闈犺浆鍙?hop銆?
func getHandshakeAgeBetween(aWGID, bWGID uint, idToPeer map[uint]*models.WireGuard, policy RoutingPolicy) (time.Duration, bool) {
	ageA, okA := getOneWayHandshakeAge(aWGID, bWGID, idToPeer, policy)
	ageB, okB := getOneWayHandshakeAge(bWGID, aWGID, idToPeer, policy)
	if !okA && !okB {
		return 0, false
	}
	if !okA {
		return ageB, true
	}
	if !okB {
		return ageA, true
	}
	if ageA >= ageB {
		return ageA, true
	}
	return ageB, true
}

// getOneWayHandshakeAge 浠?fromWGID 鐨?runtimeInfo 涓煡鍒?toWGID 瀵瑰簲 peer 鐨?last_handshake_time_sec/nsec锛岃繑鍥炴彙鎵嬧€滆窛绂荤幇鍦ㄢ€濈殑鏃堕棿宸€?
func getOneWayHandshakeAge(fromWGID, toWGID uint, idToPeer map[uint]*models.WireGuard, policy RoutingPolicy) (time.Duration, bool) {
	if policy.NetworkTopologyCache == nil {
		return 0, false
	}
	toPeer := idToPeer[toWGID]
	if toPeer == nil || toPeer.ClientID == "" {
		return 0, false
	}
	runtimeInfo, ok := policy.NetworkTopologyCache.GetRuntimeInfo(fromWGID)
	if !ok || runtimeInfo == nil {
		return 0, false
	}
	var hsSec uint64
	var hsNsec uint64
	for _, p := range runtimeInfo.GetPeers() {
		if p == nil {
			continue
		}
		if p.GetClientId() != toPeer.ClientID {
			continue
		}
		hsSec = p.GetLastHandshakeTimeSec()
		hsNsec = p.GetLastHandshakeTimeNsec()
		break
	}
	if hsSec == 0 {
		return 0, false
	}
	t := time.Unix(int64(hsSec), int64(hsNsec))
	age := time.Since(t)
	if age < 0 {
		age = 0
	}
	return age, true
}
