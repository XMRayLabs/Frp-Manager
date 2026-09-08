package streamlog

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/sourcegraph/conc"
)

func GetLogHandler(appInstance app.Application) func(*gin.Context) {
	return func(c *gin.Context) {
		getLogHander(c, appInstance)
	}
}

func getLogHander(c *gin.Context, appInstance app.Application) {
	id := c.Query("id")
	pkgsQuery := c.Query("pkgs")
	pkgs := strings.Split(pkgsQuery, ",")
	logger.Logger(c).Infof("user try to get stream log, id: [%s], pkgs: [%s]", id, pkgsQuery)

	if id == "" {
		c.JSON(http.StatusBadRequest, common.Err("id is empty"))
		return
	}
	if err := dao.CanAccessClient(app.NewContext(c, appInstance), common.GetUserInfo(c), id, defs.RBACActionView); err != nil {
		c.JSON(http.StatusForbidden, common.Err("permission denied"))
		return
	}

	if len(pkgs) != 0 {
		if pkgs[0] == "all" {
			pkgs = make([]string, 0)
		}
	}

	appInstance.GetClientLogManager().GetClientLock(id).Lock()
	defer appInstance.GetClientLogManager().GetClientLock(id).Unlock()

	ch := make(chan string, CacheBufSize)
	if oldCh, ok := appInstance.GetClientLogManager().LoadAndDelete(id); ok {
		close(oldCh)
	}
	appInstance.GetClientLogManager().Store(id, ch)

	_, err := rpc.CallClient(app.NewContext(c, appInstance), id, pb.Event_EVENT_START_STREAM_LOG, &pb.StartSteamLogRequest{Pkgs: pkgs})
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.Err(err.Error()))
		return
	}

	defer func() {
		appInstance.GetClientLogManager().Delete(id)
		close(ch)
		rpc.CallClient(app.NewContext(context.Background(), appInstance), id, pb.Event_EVENT_STOP_STREAM_LOG, &pb.CommonRequest{})
	}()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Content-Encoding", "none")
	c.Writer.Flush()

	var wg conc.WaitGroup

	wg.Go(func() {
		for l := range ch {
			k, _ := json.Marshal(l)
			_, err := c.Writer.WriteString(string(k) + "\r\n")
			if err != nil {
				logger.Logger(c).Errorf("write log error: %v", err)
				break
			}
			c.Writer.Flush()
		}
	})

	select {
	case <-c.Request.Context().Done():
		return
	case <-c.Writer.CloseNotify():
		return
	}
}
