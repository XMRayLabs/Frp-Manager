//go:build !windows
// +build !windows

package wg

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/wg/multibind"
	"github.com/Sakurame1/frp-manager/utils"
)

var (
	_ app.WireGuard = (*wireGuard)(nil)
)

type wireGuard struct {
	sync.RWMutex

	ifce            *defs.WireGuardConfig
	endpointPingMap *utils.SyncMap[uint32, uint32] // ms
	virtAddrPingMap *utils.SyncMap[string, uint32] // ms
	// ping 骞虫粦鍣細瀵光€滅灛鏃舵帰娴嬪€尖€濆仛 EWMA 鑱氬悎锛岄檷浣庢姈鍔?
	pingAggMu         sync.Mutex
	endpointPingEWMA  map[uint32]float64 // peerID -> ema(ms)
	virtAddrPingEWMA  map[string]float64 // virtAddr -> ema(ms)
	peerDirectory   map[uint32]*pb.WireGuardPeerConfig
	// 浠呯敤浜庘€滈杩炴帴/淇濇寔杩炴帴鈥濈殑 peer锛圓llowedIPs 涓虹┖锛夛紝鐢ㄤ簬鍚庣画鏍规嵁鎷撴墤鍙樺寲鍋氬鍒?
	preconnectPeers map[uint32]struct{}

	wgDevice  *device.Device
	tunDevice tun.Device
	multiBind *multibind.MultiBind
	gvisorNet *netstack.Net
	fwManager *firewallManager

	running      bool
	useGvisorNet bool // if true, use gvisor netstack

	svcLogger *logrus.Entry
	ctx       *app.Context
	cancel    context.CancelFunc
}
