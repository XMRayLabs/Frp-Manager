package server

import (
	"encoding/json"
	"fmt"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

func FRPAuth(ctx *app.Context, req *pb.FRPAuthRequest) (*pb.FRPAuthResponse, error) {
	reject := func(err error) (*pb.FRPAuthResponse, error) {
		return &pb.FRPAuthResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()}, Ok: false}, err
	}
	if req == nil {
		return reject(fmt.Errorf("invalid FRP authentication request"))
	}
	server, err := ValidateServerRequest(ctx, req.GetBase())
	if err != nil {
		return reject(fmt.Errorf("FRP server authentication failed; update the server kernel"))
	}
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	user, err := dao.AuthenticateFRPUser(db, req.GetUser(), req.GetToken())
	if err != nil {
		return reject(err)
	}
	if err := dao.CanUseServer(db, user, server.ServerID); err != nil {
		return reject(err)
	}
	switch req.GetOperation() {
	case "Login", "Ping", "NewWorkConn":
	case "NewProxy":
		allowed, err := dao.ServerAllowedPorts(db, user, server.ServerID)
		if err != nil {
			return reject(err)
		}
		content, _ := json.Marshal(map[string]interface{}{"type": req.GetProxyType(), "remotePort": req.GetRemotePort()})
		if err := models.ValidateProxyAllowedPorts(allowed, content, server.ConfigContent); err != nil {
			return reject(err)
		}
	default:
		return reject(fmt.Errorf("unsupported FRP authentication operation; update the server kernel"))
	}
	return &pb.FRPAuthResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"}, Ok: true}, nil
}
