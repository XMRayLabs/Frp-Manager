package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/frpauth"
	"github.com/Sakurame1/frp-manager/utils/logger"
	plugin "github.com/fatedier/frp/pkg/plugin/server"
	"github.com/gin-gonic/gin"
)

func NewRouter(appInstance app.Application) *gin.Engine {
	router := gin.Default()
	router.POST("/auth", MakeGinHandlerFunc(appInstance, HandleLogin))
	return router
}

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
		res, err := handler(app.NewContext(ctx, appInstance))
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
	req.Base = ctx.GetApp().GetServerBase()
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	response, err := ctx.GetApp().GetMasterCli().Call().FRPCAuth(callCtx, req)
	if err != nil || !response.GetOk() {
		return plugin.Response{Reject: true, RejectReason: "FRP account, server or port authorization denied"}, nil
	}
	return plugin.Response{Unchange: true}, nil
}
