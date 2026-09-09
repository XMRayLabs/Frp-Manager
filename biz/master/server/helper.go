package server

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
)

type ValidateableServerRequest interface {
	GetServerSecret() string
	GetServerId() string
}

func ValidateServerRequest(ctx *app.Context, req ValidateableServerRequest) (*models.ServerEntity, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request")
	}

	if req.GetServerId() == "" || req.GetServerSecret() == "" {
		return nil, fmt.Errorf("invalid request")
	}

	var (
		cli *models.ServerEntity
		err error
	)

	if cli, err = dao.NewQuery(ctx).ValidateServerSecret(req.GetServerId(), req.GetServerSecret()); err != nil {
		return nil, err
	}

	if message, ok := req.(proto.Message); ok {
		reflection := message.ProtoReflect()
		field := reflection.Descriptor().Fields().ByName("server_id")
		if field != nil {
			reflection.Set(field, protoreflect.ValueOfString(cli.ServerID))
		}
	}
	return cli, nil
}
