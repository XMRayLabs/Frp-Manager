package auth

import (
	"net/http"

	serverauth "github.com/Sakurame1/frp-manager/biz/master/server"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/frpauth"
	"github.com/Sakurame1/frp-manager/utils/logger"
	plugin "github.com/fatedier/frp/pkg/plugin/server"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Msg string `json:"msg"`
}

type HTTPError struct {
	Code int
	Err  error
}

func (e *HTTPError) Error() string {
	return e.Err.Error()
}

type HandlerFunc func(ctx *app.Context) (interface{}, error)

func MakeGinHandlerFunc(appInstance app.Application, handler HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		appCtx := app.NewContext(ctx, appInstance)
		res, err := handler(appCtx)
		if err != nil {
			logger.Logger(ctx).Infof("handle %s error: %v", ctx.Request.URL.Path, err)
			switch e := err.(type) {
			case *HTTPError:
				ctx.JSON(e.Code, &Response{Msg: e.Err.Error()})
			default:
				ctx.JSON(500, &Response{Msg: err.Error()})
			}
			return
		}
		ctx.JSON(http.StatusOK, res)
	}
}

func HandleLogin(ctx *app.Context) (interface{}, error) {
	req, err := frpauth.Decode(ctx.GetGinCtx().Request.Body)
	if err != nil {
		return plugin.Response{Reject: true, RejectReason: "invalid FRP authentication request"}, nil
	}
	node, err := dao.NewQuery(ctx).GetDefaultServer()
	if err != nil {
		return plugin.Response{Reject: true, RejectReason: "FRP server unavailable"}, nil
	}
	req.Base = &pb.ServerBase{ServerId: node.ServerID, ServerSecret: node.ConnectSecret}
	res, err := serverauth.FRPAuth(ctx, req)
	if err != nil || !res.GetOk() {
		return plugin.Response{Reject: true, RejectReason: "FRP account, server or port authorization denied"}, nil
	}
	return plugin.Response{Unchange: true}, nil
}
