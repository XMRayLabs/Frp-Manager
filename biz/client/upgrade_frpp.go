package client

import (
	bizupgrade "github.com/Sakurame1/frp-manager/biz/common/upgrade"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
)

func UpgradeFrpp(ctx *app.Context, req *pb.UpgradeFrppRequest) (*pb.UpgradeFrppResponse, error) {
	return bizupgrade.RemoteUpgrade(ctx, req)
}
