package rpc

import (
	"context"
	"fmt"
	"io"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func CallClientWrapper[R common.RespType](c *app.Context, clientID string, event pb.Event, req proto.Message, resp *R) error {
	cresp, err := CallClient(c, clientID, event, req)
	if err != nil {
		return err
	}

	protoMsgRef, ok := any(resp).(protoreflect.ProtoMessage)
	if !ok {
		return fmt.Errorf("type does not implement protoreflect.ProtoMessage")
	}

	return proto.Unmarshal(cresp.GetData(), protoMsgRef)
}

func CallClient(ctx *app.Context, clientID string, event pb.Event, msg proto.Message) (*pb.ClientMessage, error) {
	sender := ctx.GetApp().GetClientsManager().Get(clientID)
	if sender == nil {
		logger.Logger(ctx).Errorf("cannot get client, id: [%s]", clientID)
		return nil, fmt.Errorf("cannot get client, id: [%s]", clientID)
	}

	// Old kernels index live FRP instances by their original IDs.
	wireMessage := proto.Clone(msg)
	if wireMessage != nil {
		rewriteRuntimeIDs(wireMessage.ProtoReflect(), func(kind, id string) string {
			if kind == "client" && id == clientID {
				return sender.CliID
			}
			if kind == "server" {
				return models.RuntimeNodeID(ctx.GetApp().GetDBManager().GetDefaultDB(), kind, id)
			}
			return id
		})
	}
	data, err := proto.Marshal(wireMessage)
	if err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot marshal")
		return nil, err
	}

	req := &pb.ServerMessage{
		Event:     event,
		Data:      data,
		SessionId: uuid.New().String(),
		ClientId:  clientID,
	}

	respCh := make(chan *pb.ClientMessage, 1)
	ctx.GetApp().GetClientRecvMap().Store(req.SessionId, respCh)
	defer ctx.GetApp().GetClientRecvMap().Delete(req.SessionId)
	sender.SendMu.Lock()
	err = sender.Conn.Send(req)
	sender.SendMu.Unlock()
	if err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot send")
		ctx.GetApp().GetClientsManager().RemoveIfCurrent(clientID, sender)
		return nil, err
	}
	var resp *pb.ClientMessage
	select {
	case resp = <-respCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if resp.Event == pb.Event_EVENT_ERROR {
		return nil, fmt.Errorf("client return error: %s", resp.Data)
	}
	return resp, nil
}

func Recv(appInstance app.Application, clientID string) chan bool {
	done := make(chan bool)
	go func() {
		c := context.Background()
		log := logger.Logger(c).WithField("clientID", clientID)
		for {
			reciver := appInstance.GetClientsManager().Get(clientID)
			if reciver == nil {
				log.Errorf("cannot get client")
				done <- true
				return
			}
			resp, err := reciver.Conn.Recv()
			if err == io.EOF {
				log.Infof("finish client recv")
				done <- true
				return
			}
			if err != nil {
				log.WithError(err).Errorf("cannot recv, usually means client disconnect")
				done <- true
				return
			}

			respChAny, ok := appInstance.GetClientRecvMap().Load(resp.SessionId)
			if !ok {
				log.Debugf("response session expired or was already handled: %s", resp.SessionId)
				continue
			}

			respCh, ok := respChAny.(chan *pb.ClientMessage)
			if !ok {
				log.Errorf("cannot cast")
				continue
			}
			log.Debugf("recv success, resp: %+v", resp)
			respCh <- resp
		}
	}()
	return done
}

func rewriteRuntimeIDs(message protoreflect.Message, rewrite func(string, string) string) {
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			return true
		}
		if field.IsList() {
			if field.Kind() == protoreflect.MessageKind {
				list := value.List()
				for i := 0; i < list.Len(); i++ {
					rewriteRuntimeIDs(list.Get(i).Message(), rewrite)
				}
			}
			return true
		}
		if field.Kind() == protoreflect.MessageKind {
			rewriteRuntimeIDs(value.Message(), rewrite)
			return true
		}
		if field.Kind() == protoreflect.StringKind {
			kind := ""
			if field.Name() == "client_id" {
				kind = "client"
			}
			if field.Name() == "server_id" {
				kind = "server"
			}
			if kind != "" {
				message.Set(field, protoreflect.ValueOfString(rewrite(kind, value.String())))
			}
		}
		return true
	})
}
