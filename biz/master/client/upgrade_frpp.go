package client

import (
	"github.com/Sakurame1/frp-manager/biz/master/node"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
)

func UpgradeFrppHandler(ctx *app.Context, req *pb.UpgradeFrppRequest) (*pb.UpgradeFrppResponse, error) {
	return node.Upgrade(ctx, req, defs.CliTypeClient)
}
