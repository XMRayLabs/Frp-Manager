package server

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"google.golang.org/protobuf/proto"
)

func HandleServerMessage(appInstance app.Application, req *pb.ServerMessage) *pb.ClientMessage {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("\n--------------------\ncatch panic !!! \nhandle server message error: %v, stack: %s\n--------------------\n", err, debug.Stack())
		}
	}()

	ctx := context.Background()
	logger.Logger(ctx).Infof("client get a server message, origin is: [%+v]", req)

	switch req.Event {
	case pb.Event_EVENT_UPDATE_FRPS:
		return app.WrapperServerMsg(appInstance, req, UpdateFrpsHander)
	case pb.Event_EVENT_REMOVE_FRPS:
		return app.WrapperServerMsg(appInstance, req, RemoveFrpsHandler)
	case pb.Event_EVENT_START_STREAM_LOG:
		return app.WrapperServerMsg(appInstance, req, StartSteamLogHandler)
	case pb.Event_EVENT_STOP_STREAM_LOG:
		return app.WrapperServerMsg(appInstance, req, StopSteamLogHandler)
	case pb.Event_EVENT_START_PTY_CONNECT:
		return app.WrapperServerMsg(appInstance, req, StartPTYConnect)
	case pb.Event_EVENT_PING:
		// Old panels send an empty ping. Only opt-in health probes inspect the core.
		var ping pb.CommonRequest
		if proto.Unmarshal(req.GetData(), &ping) == nil && ping.GetData() == "core-health" {
			ctrl := appInstance.GetServerController()
			if ctrl == nil || ctrl.Get(appInstance.GetConfig().Client.ID) == nil {
				return &pb.ClientMessage{Event: pb.Event_EVENT_ERROR, Data: []byte("frps core is not configured or did not start")}
			}
			if core, ok := ctrl.Get(appInstance.GetConfig().Client.ID).(interface{ Running() bool }); ok && !core.Running() {
				return &pb.ClientMessage{Event: pb.Event_EVENT_ERROR, Data: []byte("frps core is not running")}
			}
		}
		rawData, _ := proto.Marshal(conf.GetVersion().ToProto())
		return &pb.ClientMessage{
			Event: pb.Event_EVENT_PONG,
			Data:  rawData,
		}
	default:
	}

	return &pb.ClientMessage{
		Event: pb.Event_EVENT_ERROR,
		Data:  []byte("unknown event"),
	}
}
