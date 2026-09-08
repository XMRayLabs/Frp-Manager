//go:build !windows
// +build !windows

package wg

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
)

const (
	endpointPingCount   = 5
	endpointPingTimeout = 10 * time.Second
)

// ping 骞虫粦鍙傛暟锛堢敤浜庝笂鎶ュ埌 master 鐨?runtimeInfo.PingMap / VirtAddrPingMap锛?
const (
	pingSmoothAlpha           = 0.30 // EWMA: new*alpha + old*(1-alpha)
	pingSmoothMinMs    uint32 = 1
	pingSmoothMaxMs    uint32 = 1500
	pingSmoothBucketMs uint32 = 5 // 鍒嗘《锛氬噺灏?1-2ms 鐨勬姈鍔ㄥ紩璧风殑娉㈠姩
)

func (w *wireGuard) pingPeers() {
	log := w.svcLogger.WithField("op", "pingPeers")

	ifceConfig, err := w.GetIfceConfig()
	if err != nil {
		log.WithError(err).Errorf("failed to get interface config")
		return
	}

	log.Debugf("start to ping peers, len: %d", len(ifceConfig.Peers))

	var waitGroup conc.WaitGroup
	w.scheduleEndpointPings(log, ifceConfig, &waitGroup)
	w.scheduleVirtualAddrPings(log, ifceConfig, &waitGroup)
	w.waitPingTasks(log, &waitGroup)
}

func (w *wireGuard) scheduleEndpointPings(log *logrus.Entry, ifceConfig *defs.WireGuardConfig, waitGroup *conc.WaitGroup) {
	targets := collectEndpointPingTargets(ifceConfig)
	if len(targets) == 0 {
		return
	}

	log.Debugf("schedule endpoint pings, targets=%d", len(targets))

	for peerID, endpoint := range targets {
		peerId := peerID
		ep := endpoint
		if ep == nil {
			continue
		}

		// ws endpoint 涓嶈蛋 ICMP ping锛屾敼涓?TCP connect 鎺㈡祴锛岄伩鍏嶈鎶?涓嶅彲杈俱€?
		if endpointTypeContainsWS(ep.GetType()) {
			tcpAddr, err := endpointTCPTarget(ep)
			if err != nil {
				log.WithError(err).Errorf("failed to resolve tcp target for endpoint, peer_id=%d, endpoint=%+v", peerId, ep)
				w.storeEndpointPing(peerId, math.MaxUint32)
				continue
			}

			waitGroup.Go(func() {
				avg, err := tcpPingAvg(tcpAddr, endpointPingCount, endpointPingTimeout)
				if err != nil {
					log.WithError(err).Errorf("tcp ping endpoint [%s] failed, peer_id=%d", tcpAddr, peerId)
					w.storeEndpointPing(peerId, math.MaxUint32)
					return
				}
				avgMs := uint32(avg.Milliseconds())
				if avgMs == 0 { // 0 means bug
					avgMs = 1
				}
				w.storeEndpointPing(peerId, avgMs)
				log.Debugf("tcp ping endpoint [%s] completed, peer_id=%d", tcpAddr, peerId)
			})
			continue
		}

		host := endpointICMPHost(ep)
		if host == "" {
			continue
		}

		epPinger, err := probing.NewPinger(host)
		if err != nil {
			log.WithError(err).Errorf("failed to create pinger for %s", host)
			continue
		}

		epPinger.Count = endpointPingCount
		epPinger.Timeout = endpointPingTimeout

		epPinger.OnFinish = func(stats *probing.Statistics) {
			// stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss
			// stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt
			log.Tracef("ping stats for %s: %v", host, stats)
			avgRttMs := uint32(stats.AvgRtt.Milliseconds())
			w.storeEndpointPing(peerId, avgRttMs)
		}

		epPinger.OnRecv = func(pkt *probing.Packet) {
			log.Tracef("recv from %s", pkt.IPAddr.String())
		}

		epPinger.OnSendError = func(_ *probing.Packet, err error) {
			log.WithError(err).Errorf("failed to send packet to %s", host)
			w.storeEndpointPing(peerId, math.MaxUint32)
		}

		waitGroup.Go(func() {
			if err := epPinger.Run(); err != nil {
				log.WithError(err).Errorf("failed to run pinger for %s", host)
				w.storeEndpointPing(peerId, math.MaxUint32)
				return
			}
			log.Debugf("ping endpoint [%s] completed, peer_id=%d", host, peerId)
		})
	}
}

func (w *wireGuard) scheduleVirtualAddrPings(log *logrus.Entry, ifceConfig *defs.WireGuardConfig, waitGroup *conc.WaitGroup) {
	if w.useGvisorNet {
		return
	}

	peers := ifceConfig.Peers
	for _, peer := range peers {
		p := peer
		// 娌℃湁 AllowedIPs 鐨?peer锛堜緥濡備粎鐢ㄤ簬淇濇寔杩炴帴/棰勮繛鎺ョ殑甯搁┗ peer锛変笉鍙備笌 virt addr 鎺㈡祴锛?
		// - 姝ゆ椂鏈満閫氬父娌℃湁鍒板绔?virtIP 鐨勮矾鐢憋紝鎺㈡祴蹇呯劧澶辫触
		// - 鑻ュけ璐ュ啓鍏ヤ笉鍙揪鍝ㄥ叺锛屼細姹℃煋 master 鐨勬嫇鎵?cache锛屽鑷?SPF 璇垽涓轰笉鍙揪锛岃繘鑰屸€滃畬鍏ㄤ笉杩為€氣€?
		if p == nil || len(p.GetAllowedIps()) == 0 {
			continue
		}
		addr := p.GetVirtualIp()
		if addr == "" {
			continue
		}

		tcpAddr, err := peerVirtualWSTCPTarget(p, addr)
		if err != nil {
			log.WithError(err).Errorf("failed to build tcp target for virt addr %s", addr)
			continue
		}

		waitGroup.Go(func() {
			avg, err := tcpPingAvg(tcpAddr, endpointPingCount, endpointPingTimeout)
			if err != nil {
				log.WithError(err).Errorf("failed to tcp ping virt addr %s via %s", addr, tcpAddr)
				// 澶辫触鏃朵笉鍐欏叆涓嶅彲杈惧摠鍏碉紝閬垮厤姹℃煋鎷撴墤锛涘垹闄よ鏉¤褰曞嵆鍙洖閫€鍒?endpoint latency
				if w.virtAddrPingMap != nil {
					w.virtAddrPingMap.Delete(addr)
				}
				return
			}

			log.Tracef("tcp ping stats for %s via %s: avg=%s", addr, tcpAddr, avg)
			avgRttMs := uint32(avg.Milliseconds())
			w.storeVirtAddrPing(addr, avgRttMs)
			log.Debugf("tcp ping virt addr [%s] completed via %s", addr, tcpAddr)
		})
	}
}

func (w *wireGuard) waitPingTasks(log *logrus.Entry, waitGroup *conc.WaitGroup) {
	log.Debugf("wait for pingers to complete")
	rcs := waitGroup.WaitAndRecover()
	if rcs != nil {
		log.WithError(rcs.AsError()).Errorf("failed to wait for pingers")
	}
}

func (w *wireGuard) storeEndpointPing(peerID uint32, ms uint32) {
	if w.endpointPingMap == nil {
		return
	}
	w.endpointPingMap.Store(peerID, w.smoothEndpointPing(peerID, ms))
}

func (w *wireGuard) storeVirtAddrPing(addr string, ms uint32) {
	if w.virtAddrPingMap == nil || addr == "" {
		return
	}
	w.virtAddrPingMap.Store(addr, w.smoothVirtAddrPing(addr, ms))
}

func clampPingMs(ms uint32) uint32 {
	if ms == 0 {
		ms = 1
	}
	if ms < pingSmoothMinMs {
		return pingSmoothMinMs
	}
	if ms > pingSmoothMaxMs {
		return pingSmoothMaxMs
	}
	return ms
}

func bucketPingMs(ms uint32) uint32 {
	if pingSmoothBucketMs == 0 {
		return ms
	}
	b := pingSmoothBucketMs
	// 鍥涜垗浜斿叆鍒版渶杩戞《
	return ((ms + b/2) / b) * b
}

func (w *wireGuard) smoothEndpointPing(peerID uint32, raw uint32) uint32 {
	// 涓嶅彲杈惧摠鍏靛€硷細鐩存帴涓婃姤涓嶅彲杈撅紝浣嗕繚鐣欏巻鍙?EWMA 浠ヤ究鎭㈠鏃跺钩婊?
	if raw == math.MaxUint32 {
		return math.MaxUint32
	}
	raw = bucketPingMs(clampPingMs(raw))
	v := float64(raw)

	w.pingAggMu.Lock()
	defer w.pingAggMu.Unlock()

	if w.endpointPingEWMA == nil {
		w.endpointPingEWMA = make(map[uint32]float64, 64)
	}
	old, ok := w.endpointPingEWMA[peerID]
	if !ok || old <= 0 {
		w.endpointPingEWMA[peerID] = v
		return raw
	}
	ema := pingSmoothAlpha*v + (1.0-pingSmoothAlpha)*old
	w.endpointPingEWMA[peerID] = ema
	ms := uint32(math.Round(ema))
	ms = bucketPingMs(clampPingMs(ms))
	return ms
}

func (w *wireGuard) smoothVirtAddrPing(addr string, raw uint32) uint32 {
	if raw == math.MaxUint32 {
		return math.MaxUint32
	}
	raw = bucketPingMs(clampPingMs(raw))
	v := float64(raw)

	w.pingAggMu.Lock()
	defer w.pingAggMu.Unlock()

	if w.virtAddrPingEWMA == nil {
		w.virtAddrPingEWMA = make(map[string]float64, 64)
	}
	old, ok := w.virtAddrPingEWMA[addr]
	if !ok || old <= 0 {
		w.virtAddrPingEWMA[addr] = v
		return raw
	}
	ema := pingSmoothAlpha*v + (1.0-pingSmoothAlpha)*old
	w.virtAddrPingEWMA[addr] = ema
	ms := uint32(math.Round(ema))
	ms = bucketPingMs(clampPingMs(ms))
	return ms
}

// collectEndpointPingTargets 鏀堕泦鎵€鏈夆€滃彲鑳界洿杩炩€濈殑鑺傜偣 endpoint锛堥珮鍐呰仛锛氬彧鍏虫敞 ping 闇€瑕佺殑鐩爣闆嗗悎锛夈€?
//
// 浼樺厛绾э細
// 1) adjs[localID] 涓?link.to_endpoint锛堟樉寮忛摼璺彲鎸囧畾 endpoint锛?
// 2) peers 涓殑 peer.endpoint锛堝厹搴曪細宸蹭笅鍙?peer config 鐨?endpoint锛?
//
// 娉ㄦ剰锛氬綋鍓?runtimeInfo.ping_map 鐨?key 鍙湁 wireguardId锛屽洜姝よ繖閲屽鍚屼竴 peerID 鍙繚鐣欎竴涓?endpoint銆?
func collectEndpointPingTargets(ifceConfig *defs.WireGuardConfig) map[uint32]*pb.Endpoint {
	if ifceConfig == nil {
		return nil
	}

	targets := make(map[uint32]*pb.Endpoint, 32)

	// 1) 浠?adj 鍥鹃噷鎷匡細鏈妭鐐癸紙ifceConfig.Id锛夊彲鐩磋繛鐨勮竟鐨勭洰鏍?endpoint
	localID := ifceConfig.GetId()
	if localID != 0 {
		if adjs := ifceConfig.GetAdjs(); adjs != nil {
			if links, ok := adjs[localID]; ok && links != nil {
				for _, l := range links.GetLinks() {
					toID := l.GetToWireguardId()
					if toID == 0 || toID == localID {
						continue
					}
					if l.GetToEndpoint() == nil {
						continue
					}
					// 鏄惧紡閾捐矾浼樺厛锛氱洿鎺ヨ鐩?
					targets[toID] = l.GetToEndpoint()
				}
			}
		}
	}

	// 2) 浠?peers 鍒楄〃鍏滃簳琛ラ綈
	for _, peer := range ifceConfig.GetPeers() {
		if peer == nil {
			continue
		}
		peerID := peer.GetId()
		if peerID == 0 || peerID == localID {
			continue
		}
		if _, exists := targets[peerID]; exists {
			continue
		}
		if peer.GetEndpoint() == nil {
			continue
		}
		targets[peerID] = peer.GetEndpoint()
	}

	return targets
}

func endpointTypeContainsWS(endpointType string) bool {
	return strings.Contains(strings.ToLower(endpointType), "ws")
}

func endpointICMPHost(ep *pb.Endpoint) string {
	if ep == nil {
		return ""
	}

	// 浼樺厛浣跨敤 Host锛堝吋瀹硅€佹暟鎹級锛屽苟鍓ョ鍙兘鐨勭鍙ｃ€?
	if ep.GetHost() != "" {
		if host, _, err := net.SplitHostPort(ep.GetHost()); err == nil && host != "" {
			return host
		}
		return ep.GetHost()
	}

	// 鍏滃簳锛氫粠 Uri 鎻愬彇 hostname
	if ep.GetUri() != "" {
		u, err := url.Parse(ep.GetUri())
		if err == nil {
			if hn := u.Hostname(); hn != "" {
				return hn
			}
		}
	}

	return ""
}

func endpointTCPTarget(ep *pb.Endpoint) (string, error) {
	if ep == nil {
		return "", errors.New("nil endpoint")
	}

	// 浼樺厛浣跨敤 Uri锛坵s/wss 鍦烘櫙鏇村噯纭級
	if ep.GetUri() != "" {
		u, err := url.Parse(ep.GetUri())
		if err != nil {
			return "", errors.Join(fmt.Errorf("parse uri '%s'", ep.GetUri()), err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("empty host in uri '%s'", ep.GetUri())
		}
		if u.Port() != "" {
			return u.Host, nil // 宸插惈绔彛
		}
		port := defaultPortForScheme(u.Scheme)
		if port == 0 && ep.GetPort() != 0 {
			port = ep.GetPort()
		}
		if port == 0 {
			return "", fmt.Errorf("missing port for uri '%s'", ep.GetUri())
		}
		return net.JoinHostPort(u.Hostname(), strconv.FormatUint(uint64(port), 10)), nil
	}

	host := strings.TrimSpace(ep.GetHost())
	if host == "" {
		return "", errors.New("empty endpoint host")
	}

	// host 宸茬粡甯︾鍙?
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host, nil
	}

	port := ep.GetPort()
	if port == 0 {
		// ws 绫诲瀷鏈樉寮忛厤缃鍙ｆ椂锛岄粯璁ゆ寜 ws=80 澶勭悊
		port = 80
	}
	return net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10)), nil
}

func defaultPortForScheme(scheme string) uint32 {
	switch strings.ToLower(scheme) {
	case "ws", "http":
		return 80
	case "wss", "https":
		return 443
	default:
		return 0
	}
}

func tcpPingAvg(addr string, count int, timeout time.Duration) (time.Duration, error) {
	if count <= 0 {
		return 0, errors.New("invalid count")
	}
	if timeout <= 0 {
		return 0, errors.New("invalid timeout")
	}

	var (
		ok    int
		sum   time.Duration
		dial  = &net.Dialer{Timeout: timeout}
		sleep = 100 * time.Millisecond
	)

	for i := 0; i < count; i++ {
		start := time.Now()
		conn, err := dial.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			sum += time.Since(start)
			ok++
		}
		// 杞诲井鎶栧姩锛岄伩鍏嶇灛鏃剁獊鍒猴紱淇濇寔閫昏緫绠€鍗曪紝涓嶅紩鍏ュ叏灞€ rate limiter銆?
		if i != count-1 {
			time.Sleep(sleep)
		}
	}

	if ok == 0 {
		return 0, fmt.Errorf("all tcp probes failed for %s", addr)
	}
	return sum / time.Duration(ok), nil
}

func peerVirtualWSTCPTarget(peer *pb.WireGuardPeerConfig, virtIP string) (string, error) {
	if peer == nil {
		return "", errors.New("nil peer")
	}
	if virtIP == "" {
		return "", errors.New("empty virt ip")
	}

	wsPort := peer.GetWsListenPort()
	if wsPort == 0 {
		// 鍏煎鈥渨s 绔彛鏈崟鐙厤缃椂澶嶇敤 listen_port鈥濈殑閫昏緫锛堣 initTransports锛?
		wsPort = peer.GetListenPort()
	}
	if wsPort == 0 {
		return "", fmt.Errorf("missing ws port for peer_id=%d virt_ip=%s", peer.GetId(), virtIP)
	}

	return net.JoinHostPort(virtIP, strconv.FormatUint(uint64(wsPort), 10)), nil
}
