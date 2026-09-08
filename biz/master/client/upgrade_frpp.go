package client

import (
	"sync"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func UpgradeFrppHandler(ctx *app.Context, req *pb.UpgradeFrppRequest) (*pb.UpgradeFrppResponse, error) {
	userInfo := common.GetUserInfo(ctx)
	clientIds := req.GetClientIds()
	log := logger.Logger(ctx)
	log.Infof("upgrade frpp called, user=%v clientIds=%v", userInfo, clientIds)

	if len(clientIds) == 0 {
		return &pb.UpgradeFrppResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "client_ids is empty"},
		}, nil
	}

	// 榛樿鍊煎鐞嗭細proto3 optional 鏈缃椂 GetXXX 杩斿洖闆跺€?
	backup := true
	if req.Backup != nil {
		backup = req.GetBackup()
	}
	restartService := true
	if req.RestartService != nil {
		restartService = req.GetRestartService()
	}
	useGithubProxy := false
	if req.UseGithubProxy != nil {
		useGithubProxy = req.GetUseGithubProxy()
	}

	// 骞跺彂涓嬪彂锛屾彁楂樺 client 鎵归噺鍗囩骇閫熷害
	var (
		wg      sync.WaitGroup
		errOnce error
		mu      sync.Mutex
	)

	for _, cid := range clientIds {
		clientId := cid
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := dao.NewQuery(ctx).GetClientByClientID(userInfo, clientId)
			if err != nil {
				mu.Lock()
				if errOnce == nil {
					errOnce = err
				}
				mu.Unlock()
				return
			}

			// 姣忔鍙粰鍗曚釜 client 涓嬪彂锛堝噺灏?payload 娣锋穯锛?
			reqForClient := &pb.UpgradeFrppRequest{
				ClientIds:      []string{clientId},
				Version:        req.Version,
				DownloadUrl:    req.DownloadUrl,
				GithubProxy:    req.GithubProxy,
				UseGithubProxy: &useGithubProxy,
				HttpProxy:      req.HttpProxy,
				TargetPath:     req.TargetPath,
				Backup:         &backup,
				ServiceName:    req.ServiceName,
				RestartService: &restartService,
				Workdir:        req.Workdir,
				ServiceArgs:    req.GetServiceArgs(),
			}

			resp := &pb.UpgradeFrppResponse{}
			if err := rpc.CallClientWrapper(ctx, clientId, pb.Event_EVENT_UPGRADE_FRPP, reqForClient, resp); err != nil {
				mu.Lock()
				if errOnce == nil {
					errOnce = err
				}
				mu.Unlock()
				return
			}
		}()
	}
	wg.Wait()

	if errOnce != nil {
		log.WithError(errOnce).Error("upgrade frpp dispatch failed")
		return nil, errOnce
	}

	return &pb.UpgradeFrppResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
