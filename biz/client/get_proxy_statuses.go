package client

import (
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/samber/lo"
)

func GetProxyStatuses(c *app.Context, req *pb.GetProxyStatusesRPCRequest) (*pb.GetProxyStatusesRPCResponse, error) {
	results := make([]*pb.ProxyStatusResult, 0, len(req.GetTargets()))
	for _, target := range req.GetTargets() {
		if target == nil {
			continue
		}

		status, err := getProxyWorkingStatus(c, target.GetClientId(), target.GetServerId(), target.GetName())
		if err != nil {
			status = &pb.ProxyWorkingStatus{
				Status: lo.ToPtr("error"),
				Err:    lo.ToPtr(err.Error()),
			}
		}
		results = append(results, &pb.ProxyStatusResult{
			ProxyId:       lo.ToPtr(target.GetProxyId()),
			WorkingStatus: status,
		})
	}

	return &pb.GetProxyStatusesRPCResponse{
		Status:        &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		ProxyStatuses: results,
	}, nil
}
