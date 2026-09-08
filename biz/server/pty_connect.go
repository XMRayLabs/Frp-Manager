package server

import (
	"github.com/Sakurame1/frp-manager/biz/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
)

func StartPTYConnect(c *app.Context, req *pb.CommonRequest) (*pb.CommonResponse, error) {
	return common.StartPTYConnect(c, req, &pb.PTYClientMessage{Base: &pb.PTYClientMessage_ServerBase{
		ServerBase: &pb.ServerBase{
			ServerId:     c.GetApp().GetConfig().Client.ID,
			ServerSecret: c.GetApp().GetConfig().Client.Secret,
		},
	}})
}
