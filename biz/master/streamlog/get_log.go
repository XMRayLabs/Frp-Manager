package streamlog

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/gin-gonic/gin"
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
	query := dao.NewQuery(app.NewContext(c, appInstance))
	user := common.GetUserInfo(c)
	key := ""
	if node, err := query.GetClientByClientID(user, id); err == nil && dao.CanManageClient(app.NewContext(c, appInstance), user, id) == nil {
		key = node.DeviceID
	} else if server, err := query.GetServerByServerID(user, id); err == nil && user.IsAdmin() {
		key = server.DeviceID
	}
	if key == "" {
		c.JSON(http.StatusForbidden, common.Err("permission denied"))
		return
	}
	connector := appInstance.GetClientsManager().Get(id)

	if len(pkgs) != 0 {
		if pkgs[0] == "all" {
			pkgs = make([]string, 0)
		}
	}

	appInstance.GetClientLogManager().GetClientLock(key).Lock()
	defer appInstance.GetClientLogManager().GetClientLock(key).Unlock()

	ch := make(chan string, CacheBufSize)
	appInstance.GetClientLogManager().Store(key, ch)
	defer appInstance.GetClientLogManager().Delete(key)
	_, err := rpc.CallConnector(app.NewContext(c.Request.Context(), appInstance), id, pb.Event_EVENT_START_STREAM_LOG, &pb.StartSteamLogRequest{Pkgs: pkgs}, connector)
	if err != nil {
		c.JSON(http.StatusInternalServerError, common.Err(err.Error()))
		return
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		rpc.CallConnector(app.NewContext(stopCtx, appInstance), id, pb.Event_EVENT_STOP_STREAM_LOG, &pb.CommonRequest{}, connector)
	}()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Content-Encoding", "none")
	c.Writer.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			current, err := query.GetUserByUserID(user.GetUserID())
			if err != nil || !current.Valid() || current.SessionVersion != user.GetSessionVersion() {
				return
			}
			if !current.IsAdmin() && dao.CanManageClient(app.NewContext(c, appInstance), current, id) != nil {
				return
			}
		case l := <-ch:
			k, _ := json.Marshal(l)
			if _, err := c.Writer.WriteString(string(k) + "\r\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		case <-connector.Done:
			return
		}
	}
}
