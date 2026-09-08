package client

import (
	"context"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/sirupsen/logrus"
)

func PullWireGuards(appInstance app.Application, clientID, clientSecret string) error {
	ctx := app.NewContext(context.Background(), appInstance)
	log := ctx.Logger().WithField("op", "PullWireGuards")

	log.Debugf("start to pull wireguards belong to client, clientID: [%s]", clientID)

	cli := ctx.GetApp().GetMasterCli()
	resp, err := cli.Call().ListClientWireGuards(ctx, &pb.ListClientWireGuardsRequest{
		Base: &pb.ClientBase{
			ClientId:     clientID,
			ClientSecret: clientSecret,
		},
	})
	if err != nil {
		log.WithError(err).Errorf("cannot list client wireguards, do not change anything")
		return err
	}

	if len(resp.GetWireguardConfigs()) == 0 {
		log.Debugf("client [%s] has no wireguards", clientID)
		return nil
	}

	log.Debugf("client [%s] has [%d] wireguards, check their status", clientID, len(resp.GetWireguardConfigs()))
	log.Tracef("wireguardConfigs: %s", resp.String())

	wgMgr := ctx.GetApp().GetWireGuardManager()
	successCnt := 0
	for _, wireGuard := range resp.GetWireguardConfigs() {
		wgCfg := &defs.WireGuardConfig{WireGuardConfig: wireGuard}
		wgSvc, ok := wgMgr.GetService(wireGuard.GetInterfaceName())
		if ok {
			if wgSvc.NeedRecreate(wgCfg) {
				wgMgr.RemoveService(wireGuard.GetInterfaceName())
			} else {
				log.Debugf("wireguard [%s] already exists, skip create, update peers if need", wireGuard.GetInterfaceName())
				syncExistingWireGuard(log, wgSvc, wgCfg)
				continue
			}
		}

		wgSvc, err := wgMgr.CreateService(&defs.WireGuardConfig{WireGuardConfig: wireGuard})
		if err != nil {
			log.WithError(err).Errorf("create wireguard service failed")
			continue
		}
		err = wgSvc.Start()
		if err != nil {
			log.WithError(err).Errorf("start wireguard service failed")
			continue
		}
		successCnt++
	}

	log.Debugf("pull wireguards belong to client success, clientID: [%s], [%d] wireguards created", clientID, successCnt)

	return nil
}

func syncExistingWireGuard(log *logrus.Entry, wgSvc app.WireGuard, wgCfg *defs.WireGuardConfig) {
	if wgSvc == nil || wgCfg == nil {
		return
	}
	// 涓婚摼璺細鍏堟洿鏂?adjs锛屽啀 patch peers銆倃g 鍐呴儴浼氬熀浜庢渶鏂版嫇鎵戝仛棰勮繛鎺ヨˉ榻?涓嶅彲鐩磋繛娓呯悊銆?
	if err := wgSvc.UpdateAdjs(wgCfg.GetAdjs()); err != nil {
		log.WithError(err).Warn("update adjs failed while syncing existing wireguard")
		return
	}
	if _, err := wgSvc.PatchPeers(wgCfg.GetParsedPeers()); err != nil {
		log.WithError(err).Warn("patch peers failed while syncing existing wireguard")
		return
	}
}
