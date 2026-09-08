package client

import (
	"github.com/Sakurame1/frp-manager/biz/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
)

func StartPTYConnect(c *app.Context, req *pb.CommonRequest) (*pb.CommonResponse, error) {
	return common.StartPTYConnect(c, req, &pb.PTYClientMessage{Base: &pb.PTYClientMessage_ClientBase{
		ClientBase: &pb.ClientBase{
			ClientId:     c.GetApp().GetConfig().Client.ID,
			ClientSecret: c.GetApp().GetConfig().Client.Secret,
		},
	}})
}
