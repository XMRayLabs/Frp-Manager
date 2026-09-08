package wg

import (
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
)

var (
	_ app.NetworkTopologyCache = (*networkTopologyCache)(nil)
)

// networkTopologyCache 鐩墠鍙粰鏈嶅姟绔敤
type networkTopologyCache struct {
	wireguardRuntimeInfoMap *utils.SyncMap[uint, *pb.WGDeviceRuntimeInfo]      // wireguardId -> peerRuntimeInfo
	fromToLatencyMap        *utils.SyncMap[uint, *utils.SyncMap[uint, uint32]] // fromWGID -> (toWGID -> latencyMs)
	virtAddrPingMap         *utils.SyncMap[uint, *utils.SyncMap[uint, uint32]] // fromWGID -> (toWGID -> pingMs)
}

func NewNetworkTopologyCache() *networkTopologyCache {
	return &networkTopologyCache{
		wireguardRuntimeInfoMap: &utils.SyncMap[uint, *pb.WGDeviceRuntimeInfo]{},
		fromToLatencyMap:        &utils.SyncMap[uint, *utils.SyncMap[uint, uint32]]{},
		virtAddrPingMap:         &utils.SyncMap[uint, *utils.SyncMap[uint, uint32]]{},
	}
}

func (c *networkTopologyCache) GetRuntimeInfo(wireguardId uint) (*pb.WGDeviceRuntimeInfo, bool) {
	return c.wireguardRuntimeInfoMap.Load(wireguardId)
}

func (c *networkTopologyCache) SetRuntimeInfo(wireguardId uint, runtimeInfo *pb.WGDeviceRuntimeInfo) {
	c.wireguardRuntimeInfoMap.Store(wireguardId, runtimeInfo)

	newFromToLatency := &utils.SyncMap[uint, uint32]{}
	for toWireGuardId, latencyMs := range runtimeInfo.GetPingMap() {
		newFromToLatency.Store(uint(toWireGuardId), latencyMs)
	}
	c.fromToLatencyMap.Store(wireguardId, newFromToLatency)

	newVirtAddrPing := &utils.SyncMap[uint, uint32]{}
	for virtAddr, pingMs := range runtimeInfo.GetVirtAddrPingMap() {
		if toWireGuardId, exists := runtimeInfo.PeerVirtAddrMap[virtAddr]; exists {
			newVirtAddrPing.Store(uint(toWireGuardId), pingMs)
		}
	}
	c.virtAddrPingMap.Store(wireguardId, newVirtAddrPing)
}

func (c *networkTopologyCache) DeleteRuntimeInfo(wireguardId uint) {
	c.wireguardRuntimeInfoMap.Delete(wireguardId)
	c.fromToLatencyMap.Delete(wireguardId)
	c.virtAddrPingMap.Delete(wireguardId)
}

func (c *networkTopologyCache) GetLatencyMs(fromWGID, toWGID uint) (uint32, bool) {
	// 灏濊瘯浠?fromWGID -> toWGID 鏂瑰悜鏌ヨ
	endpointLatency, endpointOk := c.getLatencyFromMap(c.fromToLatencyMap, fromWGID, toWGID)
	if !endpointOk {
		// 灏濊瘯鍙嶅悜鏌ヨ toWGID -> fromWGID
		endpointLatency, endpointOk = c.getLatencyFromMap(c.fromToLatencyMap, toWGID, fromWGID)
		if !endpointOk {
			return 0, false
		}
	}

	// 灏濊瘯浠?fromWGID -> toWGID 鏂瑰悜鏌ヨ铏氭嫙鍦板潃寤惰繜
	virtAddrLatency, virtAddrOk := c.getLatencyFromMap(c.virtAddrPingMap, fromWGID, toWGID)
	if !virtAddrOk {
		// 灏濊瘯鍙嶅悜鏌ヨ
		virtAddrLatency, virtAddrOk = c.getLatencyFromMap(c.virtAddrPingMap, toWGID, fromWGID)
		if !virtAddrOk {
			return endpointLatency, true
		}
	}

	return (endpointLatency + virtAddrLatency) / 2, true
}

// getLatencyFromMap 浠庡祵濂?map 涓煡璇㈠欢杩熷€?
func (c *networkTopologyCache) getLatencyFromMap(m *utils.SyncMap[uint, *utils.SyncMap[uint, uint32]], fromWGID, toWGID uint) (uint32, bool) {
	innerMap, ok := m.Load(fromWGID)
	if !ok {
		return 0, false
	}
	return innerMap.Load(toWGID)
}
