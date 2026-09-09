package streamlog

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/Sakurame1/frp-manager/biz/master/client"
	"github.com/Sakurame1/frp-manager/biz/master/server"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

const (
	CacheBufSize = 4096
)

type ClientLogManager struct {
	*utils.SyncMap[string, chan string]
	clientLocksMap *utils.SyncMap[string, *sync.Mutex]
}

func (c *ClientLogManager) GetClientLock(clientId string) *sync.Mutex {
	lock, _ := c.clientLocksMap.LoadOrStore(clientId, &sync.Mutex{})
	return lock
}

func NewClientLogManager() app.ClientLogManager {
	return &ClientLogManager{
		SyncMap:        &utils.SyncMap[string, chan string]{},
		clientLocksMap: &utils.SyncMap[string, *sync.Mutex]{},
	}
}

func PushClientStreamLog(ctx *app.Context, sender pb.Master_PushClientStreamLogServer) error {
	for {
		req, err := sender.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot recv from client, id: [%+v]", req.GetBase())
			return err
		}

		node, err := client.ValidateClientRequest(ctx, req.GetBase())
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot validate client, id: [%+v]", req.GetBase())
			return err
		}

		ch, ok := ctx.GetApp().GetClientLogManager().Load(node.DeviceID)
		if !ok {
			return fmt.Errorf("push client stream log cannot find client, id: [%s]", req.GetBase().GetClientId())
		}

		select {
		case ch <- string(req.GetLog()):
		case <-sender.Context().Done():
			return sender.Context().Err()
		default:
		}
	}
	return nil
}

func PushServerStreamLog(ctx *app.Context, sender pb.Master_PushServerStreamLogServer) error {
	for {
		req, err := sender.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot recv from server, req: [%+v]", req.GetBase())
			return err
		}

		node, err := server.ValidateServerRequest(ctx, req.GetBase())
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("cannot validate server, req: [%+v]", req.GetBase())
			return err
		}

		ch, ok := ctx.GetApp().GetClientLogManager().Load(node.DeviceID)
		if !ok {
			return fmt.Errorf("push server stream log cannot find server, id: [%s]", req.GetBase().GetServerId())
		}
		select {
		case ch <- string(req.GetLog()):
		case <-sender.Context().Done():
			return sender.Context().Err()
		default:
		}
	}
	return nil
}
