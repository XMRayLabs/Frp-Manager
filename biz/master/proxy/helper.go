package proxy

import (
	"errors"

	"github.com/Sakurame1/frp-manager/biz/master/client"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func convertProxyStatsList(proxyList []*models.ProxyStatsEntity) []*pb.ProxyInfo {
	return lo.Map(proxyList, func(item *models.ProxyStatsEntity, index int) *pb.ProxyInfo {
		return &pb.ProxyInfo{
			Name:              lo.ToPtr(item.Name),
			Type:              lo.ToPtr(item.Type),
			ClientId:          lo.ToPtr(item.ClientID),
			OriginClientId:    lo.ToPtr(item.OriginClientID),
			ServerId:          lo.ToPtr(item.ServerID),
			TodayTrafficIn:    lo.ToPtr(item.TodayTrafficIn),
			TodayTrafficOut:   lo.ToPtr(item.TodayTrafficOut),
			HistoryTrafficIn:  lo.ToPtr(item.HistoryTrafficIn),
			HistoryTrafficOut: lo.ToPtr(item.HistoryTrafficOut),
		}
	})
}

// GetClientWithMakeShadow
// 1. 妫€鏌ユ槸鍚︽湁宸茶繛鎺ヨ鏈嶅姟绔殑瀹㈡埛绔?
// 2. 妫€鏌ユ槸鍚︽湁Shadow瀹㈡埛绔?
// 3. 濡傛灉娌℃湁锛屽垯鏂板缓Shadow瀹㈡埛绔拰瀛愬鎴风
func GetClientWithMakeShadow(c *app.Context, clientID, serverID string) (*models.ClientEntity, error) {
	userInfo := common.GetUserInfo(c)
	clientEntity, err := dao.NewQuery(c).GetClientByFilter(userInfo, &models.ClientEntity{OriginClientID: clientID, ServerID: serverID}, lo.ToPtr(false))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		clientEntity, err = dao.NewQuery(c).GetClientByFilter(userInfo, &models.ClientEntity{ClientID: clientID}, nil)
		if err != nil {
			logger.Logger(c).WithError(err).Errorf("cannot get client, id: [%s]", clientID)
			return nil, err
		}
		if (!clientEntity.IsShadow || len(clientEntity.ConfigContent) != 0) && len(clientEntity.OriginClientID) == 0 {
			// 娌hadow杩囷紝闇€瑕乻hadow
			_, err = client.MakeClientShadowed(c, serverID, clientEntity)
			if err != nil {
				logger.Logger(c).WithError(err).Errorf("cannot make client shadow, id: [%s]", clientID)
				return nil, err
			}
		}
		// shadow杩囷紝浣嗘病鎵惧埌瀛愬鎴风锛岄渶瑕佹柊寤?
		clientEntity, _, err = client.ChildClientForServer(c, serverID, clientEntity)
		if err != nil {
			logger.Logger(c).WithError(err).Errorf("cannot create child client, id: [%s]", clientID)
			return nil, err
		}
	}
	// 鏈変换浣曞け璐ワ紝杩斿洖
	if err != nil {
		logger.Logger(c).WithError(err).Errorf("cannot get client, id: [%s]", clientID)
		return nil, err
	}

	return clientEntity, nil
}
