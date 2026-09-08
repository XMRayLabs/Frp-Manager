package wg

import (
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/samber/lo"
)

type fakeTopologyCache struct {
	lat map[[2]uint]uint32
	rt  map[uint]*pb.WGDeviceRuntimeInfo
}

func (c *fakeTopologyCache) GetRuntimeInfo(id uint) (*pb.WGDeviceRuntimeInfo, bool) {
	if c == nil || c.rt == nil {
		return nil, false
	}
	v, ok := c.rt[id]
	return v, ok
}
func (c *fakeTopologyCache) SetRuntimeInfo(_ uint, _ *pb.WGDeviceRuntimeInfo) {}
func (c *fakeTopologyCache) DeleteRuntimeInfo(_ uint)                         {}
func (c *fakeTopologyCache) GetLatencyMs(fromWGID, toWGID uint) (uint32, bool) {
	if c == nil || c.lat == nil {
		return 0, false
	}
	v, ok := c.lat[[2]uint{fromWGID, toWGID}]
	return v, ok
}

func TestFilterAdjacencyForSPF(t *testing.T) {
	cache := &fakeTopologyCache{
		lat: map[[2]uint]uint32{
			{1, 2}: 10,
			{2, 1}: 10,
			{1, 4}: ^uint32(0), // MaxUint32: 鏄庣‘涓嶅彲杈撅紝搴旇鍓旈櫎
		},
	}

	policy := RoutingPolicy{NetworkTopologyCache: cache}

	adj := map[uint][]Edge{
		1: {
			{to: 2, latency: 30, explicit: false},              // implicit + 鏈夋帰娴?=> 淇濈暀锛屼笖 latency 瑕嗙洊涓?10
			{to: 3, latency: 30, explicit: false},              // implicit + 鏃犳帰娴?=> 鍓旈櫎
			{to: 4, latency: 30, explicit: false},              // implicit + 涓嶅彲杈惧摠鍏?=> 鍓旈櫎
			{to: 5, latency: 1, upMbps: 1, explicit: true},     // explicit => 淇濈暀
			{to: 6, latency: 999, upMbps: 1, explicit: true},   // explicit => 淇濈暀
			{to: 7, latency: 30, upMbps: 50, explicit: false},  // implicit + 鏃犳帰娴?=> 鍓旈櫎
			{to: 8, latency: 30, upMbps: 50, explicit: false},  // implicit + 鏃犳帰娴?=> 鍓旈櫎
			{to: 9, latency: 30, upMbps: 50, explicit: false},  // implicit + 鏃犳帰娴?=> 鍓旈櫎
			{to: 10, latency: 30, upMbps: 50, explicit: false}, // implicit + 鏃犳帰娴?=> 鍓旈櫎
		},
	}

	ret := filterAdjacencyForSPF([]uint{1, 2}, adj, policy)

	edges1 := ret[1]
	if len(edges1) != 3 {
		t.Fatalf("want 3 edges for node 1, got %d: %#v", len(edges1), edges1)
	}

	// 鏍￠獙 implicit edge(1->2) 鐨?latency 琚鐩栦负鐪熷疄鎺㈡祴鍊?10
	found12 := false
	for _, e := range edges1 {
		if e.to == 2 {
			found12 = true
			if e.latency != 10 {
				t.Fatalf("want latency=10 for edge 1->2, got %d", e.latency)
			}
		}
		if e.to == 4 {
			t.Fatalf("edge 1->4 should be filtered out (MaxUint32)")
		}
		if e.to == 3 {
			t.Fatalf("edge 1->3 should be filtered out (no probe data)")
		}
	}
	if !found12 {
		t.Fatalf("edge 1->2 should exist")
	}

	// order 涓殑鑺傜偣蹇呴』瀛樺湪 key锛堝嵆渚挎病鏈夎竟锛?
	if _, ok := ret[2]; !ok {
		t.Fatalf("node 2 should exist in return map")
	}
}

func TestPlanAllowedIPs_PreferFreshHandshake(t *testing.T) {
	// 1 <-> 2锛氫綆寤惰繜浣嗘彙鎵嬭繃鏃э紙搴旇鎯╃綒锛?
	// 1 <-> 3 <-> 2锛氱暐楂樺欢杩熶絾鎻℃墜鏂帮紙搴旇閫変负 1->2 鐨?nextHop=3锛?
	now := time.Now().Unix()

	priv1, _ := wgtypes.GeneratePrivateKey()
	priv2, _ := wgtypes.GeneratePrivateKey()
	priv3, _ := wgtypes.GeneratePrivateKey()

	p1 := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{ClientID: "c1", PrivateKey: priv1.String(), LocalAddress: "10.0.0.1/32"}}
	p1.ID = 1
	p2 := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{ClientID: "c2", PrivateKey: priv2.String(), LocalAddress: "10.0.0.2/32"}}
	p2.ID = 2
	p3 := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{ClientID: "c3", PrivateKey: priv3.String(), LocalAddress: "10.0.0.3/32"}}
	p3.ID = 3

	// 鏄惧紡閾捐矾涔熻姹傝嚦灏戜竴渚у瓨鍦?endpoint锛堢鍚堢湡瀹炶繍琛屾椂锛氶渶瑕佸彲杩炴帴鍏ュ彛锛?
	p1.AdvertisedEndpoints = []*models.Endpoint{{EndpointEntity: &models.EndpointEntity{Host: "redacted.example", Port: 61820, Type: "ws", WireGuardID: 1, ClientID: "c1"}}}
	p2.AdvertisedEndpoints = []*models.Endpoint{{EndpointEntity: &models.EndpointEntity{Host: "redacted.example", Port: 61820, Type: "ws", WireGuardID: 2, ClientID: "c2"}}}
	p3.AdvertisedEndpoints = []*models.Endpoint{{EndpointEntity: &models.EndpointEntity{Host: "redacted.example", Port: 61820, Type: "ws", WireGuardID: 3, ClientID: "c3"}}}

	peers := []*models.WireGuard{p1, p2, p3}
	link := func(from, to uint, latency uint32) *models.WireGuardLink {
		return &models.WireGuardLink{WireGuardLinkEntity: &models.WireGuardLinkEntity{
			FromWireGuardID: from,
			ToWireGuardID:   to,
			UpBandwidthMbps: 50,
			LatencyMs:       latency,
			Active:          true,
		}}
	}
	links := []*models.WireGuardLink{
		link(1, 2, 5), link(2, 1, 5),
		link(1, 3, 8), link(3, 1, 8),
		link(3, 2, 8), link(2, 3, 8),
	}

	cache := &fakeTopologyCache{
		rt: map[uint]*pb.WGDeviceRuntimeInfo{
			1: {Peers: []*pb.WGPeerRuntimeInfo{
				{ClientId: "c2", LastHandshakeTimeSec: uint64(now - 3600)}, // stale
				{ClientId: "c3", LastHandshakeTimeSec: uint64(now)},        // fresh
			}},
			2: {Peers: []*pb.WGPeerRuntimeInfo{
				{ClientId: "c1", LastHandshakeTimeSec: uint64(now - 3600)}, // stale (瀵圭О)
				{ClientId: "c3", LastHandshakeTimeSec: uint64(now)},        // fresh
			}},
			3: {Peers: []*pb.WGPeerRuntimeInfo{
				{ClientId: "c1", LastHandshakeTimeSec: uint64(now)},
				{ClientId: "c2", LastHandshakeTimeSec: uint64(now)},
			}},
		},
	}

	policy := DefaultRoutingPolicy(NewACL(), cache, nil)
	policy.HandshakeStaleThreshold = 1 * time.Second
	policy.HandshakeStalePenalty = 1000
	policy.InverseBandwidthWeight = 0
	policy.HopWeight = 0
	policy.LatencyLogScale = 0

	peerCfgs, _, err := PlanAllowedIPs(peers, links, policy)
	if err != nil {
		t.Fatalf("PlanAllowedIPs err: %v", err)
	}

	// 瀵?node1锛?0.0.0.2/32 搴旇蛋 peer(3) 鑰屼笉鏄?peer(2)
	wantDst := "10.0.0.2/32"
	var gotPeer uint32
	for _, pc := range peerCfgs[1] {
		if pc == nil {
			continue
		}
		if lo.Contains(pc.GetAllowedIps(), wantDst) {
			gotPeer = pc.GetId()
		}
	}
	if gotPeer != 3 {
		t.Fatalf("want node1 route %s via peer 3, got peer %d", wantDst, gotPeer)
	}
}

func TestSymmetrizeAdjacencyForPeers_FillReverseEdge(t *testing.T) {
	t.Skip("symmetrizeAdjacencyForPeers 宸茬Щ闄わ細璺敱鎵胯浇鐨勮竟蹇呴』鍙屽悜瀛樺湪锛屼笉搴旇嚜鍔ㄨˉ榻愬崟鍚戣竟")
}

func TestFilterAdjacencyForSymmetricLinks_DropOneWay(t *testing.T) {
	order := []uint{1, 2}
	adj := map[uint][]Edge{
		1: {{to: 2, latency: 10, upMbps: 50, explicit: true}}, // 鍗曞悜
		2: {},
	}
	ret := filterAdjacencyForSymmetricLinks(order, adj)
	if len(ret[1]) != 0 {
		t.Fatalf("want 0 edges for node 1 after symmetric filter, got %d: %#v", len(ret[1]), ret[1])
	}
	if _, ok := ret[2]; !ok {
		t.Fatalf("node 2 should exist in return map")
	}
}

func TestEnsureRoutingPeerSymmetry_AddReversePeer(t *testing.T) {
	t.Skip("routing planner rewritten: inbound-source-set generation replaces old symmetry patching")
}

func TestPlanAllowedIPs_Regression_NoDuplicateAllowedIPs_And_TransitSourceValidation(t *testing.T) {
	// 澶嶇幇 & 闃插洖褰掞細
	// 1) 鍚屼竴鑺傜偣鐨?AllowedIPs 涓嶅厑璁稿湪澶氫釜 peer 闂撮噸澶嶏紙渚嬪 10.10.0.4/32 鍙兘鍒嗛厤缁欎竴涓?nextHop锛?
	// 2) 澶氳烦杞彂鏃讹紝鍏ョ珯 source validation 闇€瑕佸厑璁糕€滃師濮嬫簮鍦板潃鈥濓細
	//    鏋勯€?21(10.10.0.8) -> 16(10.10.0.2) 璧?24 涓浆锛?
	//    鏈熸湜 16 鐨?peer(24) AllowedIPs 鍖呭惈 10.10.0.8/32锛堝惁鍒?16 浼氫涪寮冩潵鑷?24 鐨勮浆鍙戝寘锛夈€?

	type node struct {
		id    uint
		cid   string
		addr  string
		tags  []string
		hasEP bool
	}

	nodes := []node{
		{id: 4, cid: "c4", addr: "10.10.0.4/24", tags: []string{"cn", "bj"}, hasEP: true},
		{id: 11, cid: "c11", addr: "10.10.0.1/24", tags: []string{"cn", "wh"}, hasEP: false},
		{id: 16, cid: "c16", addr: "10.10.0.2/24", tags: []string{"cn", "bj", "ali"}, hasEP: true},
		{id: 17, cid: "c17", addr: "10.10.0.3/24", tags: []string{"cn", "wh"}, hasEP: false},
		{id: 18, cid: "c18", addr: "10.10.0.6/24", tags: []string{"us"}, hasEP: true},
		{id: 20, cid: "c20", addr: "10.10.0.7/24", tags: []string{"us"}, hasEP: false},
		{id: 21, cid: "c21", addr: "10.10.0.8/24", tags: []string{"cn", "nc"}, hasEP: false},
		{id: 22, cid: "c22", addr: "10.10.0.9/24", tags: []string{"cn", "nc"}, hasEP: false},
		{id: 24, cid: "c24", addr: "10.10.0.5/24", tags: []string{"cn", "nc"}, hasEP: true},
	}

	makePeer := func(n node) *models.WireGuard {
		priv, _ := wgtypes.GeneratePrivateKey()
		wg := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{
			ClientID:     n.cid,
			PrivateKey:   priv.String(),
			LocalAddress: n.addr,
			Tags:         n.tags,
		}}
		wg.ID = n.id
		if n.hasEP {
			wg.AdvertisedEndpoints = []*models.Endpoint{
				{EndpointEntity: &models.EndpointEntity{
					Host:        "redacted.example",
					Port:        61820,
					Type:        "ws",
					WireGuardID: n.id,
					ClientID:    n.cid,
				}},
			}
		}
		return wg
	}

	peers := lo.Map(nodes, func(n node, _ int) *models.WireGuard { return makePeer(n) })

	// 鏋勯€?ACL锛堜笌鐢ㄦ埛鎻愪緵涓€鑷达細鍙獙璇?tag 鍖归厤閫昏緫姝ｇ‘锛屼笉娑夊強鍏綉淇℃伅锛?
	acl := NewACL().LoadFromPB(&pb.AclConfig{Acls: []*pb.AclRuleConfig{
		{Action: "allow", Src: []string{"bj", "wh"}, Dst: []string{"bj", "wh"}},
		{Action: "allow", Src: []string{"nc", "wh"}, Dst: []string{"nc", "wh"}},
		{Action: "allow", Src: []string{"nc", "ali"}, Dst: []string{"nc", "ali"}},
		{Action: "allow", Src: []string{"wh", "ali"}, Dst: []string{"wh", "ali"}},
		{Action: "allow", Src: []string{"us"}, Dst: []string{"us"}},
	}})

	// 鍙渶瑕?latency cache 涓烘帹鏂竟鎻愪緵鈥滄帰娴嬪瓨鍦ㄦ€р€濓紝杩欓噷鐩存帴鎵嬪姩鏋勯€犳樉寮?links锛屾洿鍙帶
	// 鍏抽敭锛氳 21->16 璧?24 涓浆锛?1-24-16 浣庡欢杩燂紝21-16 楂樺欢杩燂級
	link := func(from, to uint, latency uint32) *models.WireGuardLink {
		return &models.WireGuardLink{WireGuardLinkEntity: &models.WireGuardLinkEntity{
			FromWireGuardID: from,
			ToWireGuardID:   to,
			UpBandwidthMbps: 50,
			LatencyMs:       latency,
			Active:          true,
		}}
	}
	links := []*models.WireGuardLink{
		link(21, 24, 10), link(24, 21, 10),
		link(24, 16, 10), link(16, 24, 10),
		link(21, 16, 200), link(16, 21, 200),

		// 鍐嶈ˉ涓€浜涜繛閫氳竟锛岀‘淇濊兘绠楀嚭鍖呭惈 4 鐨勮矾鐢?
		link(11, 16, 30), link(16, 11, 30),
		link(16, 4, 5), link(4, 16, 5),
		link(11, 4, 50), link(4, 11, 50),
	}

	policy := DefaultRoutingPolicy(acl, &fakeTopologyCache{lat: map[[2]uint]uint32{}}, nil)
	policy.HandshakeStalePenalty = 0
	policy.HandshakeStaleThreshold = 0
	policy.InverseBandwidthWeight = 0
	policy.HopWeight = 0
	policy.LatencyLogScale = 0

	peerCfgs, _, err := PlanAllowedIPs(peers, links, policy)
	if err != nil {
		t.Fatalf("PlanAllowedIPs err: %v", err)
	}

	// 1) 鏂█锛氭瘡涓妭鐐圭殑 AllowedIPs 鍦ㄤ笉鍚?peer 闂翠笉閲嶅
	for owner, pcs := range peerCfgs {
		seen := map[string]uint32{}
		for _, pc := range pcs {
			if pc == nil {
				continue
			}
			for _, cidr := range pc.GetAllowedIps() {
				if prev, ok := seen[cidr]; ok && prev != pc.GetId() {
					t.Fatalf("node %d has duplicate cidr %s on peer %d and peer %d", owner, cidr, prev, pc.GetId())
				}
				seen[cidr] = pc.GetId()
			}
		}
	}

	// 2) 鏂█锛?6 鐨?peer(24) 蹇呴』鍖呭惈 10.10.0.8/32锛?1 鐨?/32锛夛紝鐢ㄤ簬鍏ョ珯 source validation
	wantSrc := "10.10.0.8/32"
	found := false
	for _, pc := range peerCfgs[16] {
		if pc == nil || pc.GetId() != 24 {
			continue
		}
		if lo.Contains(pc.GetAllowedIps(), wantSrc) {
			found = true
		}
	}
	if !found {
		t.Fatalf("node 16 peer(24) should contain %s for transit source validation", wantSrc)
	}

	// 3) 鏂█锛?1 鑺傜偣鐨?10.10.0.4/32 涓嶈兘鍚屾椂鍑虹幇鍦ㄥ涓?peer
	wantC4 := "10.10.0.4/32"
	var peersWithC4 []uint32
	for _, pc := range peerCfgs[11] {
		if pc == nil {
			continue
		}
		if lo.Contains(pc.GetAllowedIps(), wantC4) {
			peersWithC4 = append(peersWithC4, pc.GetId())
		}
	}
	if len(peersWithC4) != 1 {
		t.Fatalf("node 11 should have exactly one peer carrying %s, got peers=%v", wantC4, peersWithC4)
	}
}

func TestBuildAdjacency_InferredEdgesAreBidirectionalWhenACLAllows(t *testing.T) {
	// 鍥炲綊锛氭帹鏂竟蹇呴』鏀寔 to(with endpoint) -> from(no endpoint) 鐨勫弽鍚戣ˉ榻愶紝
	// 鍚﹀垯 filterAdjacencyForSymmetricLinks 浼氭妸鎵€鏈?鈥渘o-endpoint 鑺傜偣鈥?鍓旈櫎锛屽鑷?SPF 缁撴灉涓虹┖銆?

	privA, _ := wgtypes.GeneratePrivateKey()
	privB, _ := wgtypes.GeneratePrivateKey()

	a := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{
		ClientID:     "ca",
		PrivateKey:   privA.String(),
		LocalAddress: "10.0.0.1/24",
		Tags:         []string{"t1"},
	}}
	a.ID = 1 // no endpoint

	b := &models.WireGuard{WireGuardEntity: &models.WireGuardEntity{
		ClientID:     "cb",
		PrivateKey:   privB.String(),
		LocalAddress: "10.0.0.2/24",
		Tags:         []string{"t1"},
	}}
	b.ID = 2
	b.AdvertisedEndpoints = []*models.Endpoint{{EndpointEntity: &models.EndpointEntity{
		Host:        "redacted.example",
		Port:        61820,
		Type:        "ws",
		WireGuardID: 2,
		ClientID:    "cb",
	}}}

	idToPeer, order := buildNodeIndexSorted([]*models.WireGuard{a, b})
	acl := NewACL().LoadFromPB(&pb.AclConfig{Acls: []*pb.AclRuleConfig{
		{Action: "allow", Src: []string{"t1"}, Dst: []string{"t1"}},
	}})
	policy := DefaultRoutingPolicy(acl, &fakeTopologyCache{lat: map[[2]uint]uint32{
		{1, 2}: 10,
		{2, 1}: 10,
	}}, nil)

	adj := buildAdjacency(order, idToPeer, nil, policy)
	// 鏈熸湜锛?->2 涓?2->1 閮藉瓨鍦紙鎺ㄦ柇杈瑰弻鍚戯級
	has12 := false
	for _, e := range adj[1] {
		if e.to == 2 {
			has12 = true
		}
	}
	has21 := false
	for _, e := range adj[2] {
		if e.to == 1 {
			has21 = true
		}
	}
	if !has12 || !has21 {
		t.Fatalf("want inferred edges 1->2 and 2->1, got has12=%v has21=%v adj=%#v", has12, has21, adj)
	}
}
