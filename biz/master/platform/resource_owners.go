package platform

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/gin-gonic/gin"
)

type resourceOwners struct {
	Owners    []ownerOption  `json:"owners"`
	Resources map[string]int `json:"resources"`
}
type ownerOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func getResourceOwners(ctx *app.Context, kind string) (*resourceOwners, error) {
	if kind != "client" && kind != "proxy" {
		return nil, fmt.Errorf("invalid resource kind")
	}
	user := common.GetUserInfo(ctx)
	result := &resourceOwners{Owners: []ownerOption{}, Resources: map[string]int{}}
	ids := map[int]bool{}
	q := dao.NewQuery(ctx)
	for page := 1; ; page++ {
		count := 0
		if kind == "client" {
			rows, err := q.ListClients(user, page, 500)
			if err != nil {
				return nil, err
			}
			count = len(rows)
			for _, row := range rows {
				result.Resources[row.ClientID] = row.UserID
				ids[row.UserID] = true
			}
		} else {
			rows, err := q.ListProxyConfigsWithFilters(user, page, 500, &models.ProxyConfigEntity{})
			if err != nil {
				return nil, err
			}
			count = len(rows)
			for _, row := range rows {
				result.Resources[strconv.FormatUint(uint64(row.ID), 10)] = row.UserID
				ids[row.UserID] = true
			}
		}
		if count < 500 {
			break
		}
	}
	if len(ids) == 0 {
		return result, nil
	}
	userIDs := make([]int, 0, len(ids))
	for id := range ids {
		userIDs = append(userIDs, id)
	}
	// Return only owner IDs and names; never emails, credentials, or other users.
	var users []struct {
		UserID   int
		UserName string
	}
	if err := ctx.GetApp().GetDBManager().GetDefaultDB().Model(&models.User{}).Select("user_id,user_name").Where("user_id IN ?", userIDs).Order("user_name ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	for _, owner := range users {
		result.Owners = append(result.Owners, ownerOption{owner.UserID, owner.UserName})
		delete(ids, owner.UserID)
	}
	for id := range ids {
		result.Owners = append(result.Owners, ownerOption{id, fmt.Sprintf("用户 #%d", id)})
	}
	return result, nil
}

func ResourceOwners(instance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := common.GetUserInfo(c)
		if user == nil || !user.Valid() {
			common.ErrUnAuthorized(c, "invalid user")
			return
		}
		result, err := getResourceOwners(app.NewContext(c, instance), c.Query("kind"))
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Err(err.Error()))
			return
		}
		c.JSON(http.StatusOK, common.OK("ok").WithBody(result))
	}
}
