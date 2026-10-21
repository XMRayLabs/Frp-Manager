package server

import (
	"strings"
	"testing"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
)

func TestUpgradeEventReachesValidatedHandler(t *testing.T) {
	data, err := proto.Marshal(&pb.UpgradeFrppRequest{Version: lo.ToPtr("../../invalid")})
	if err != nil {
		t.Fatal(err)
	}
	response := HandleServerMessage(app.NewApp(), &pb.ServerMessage{Event: pb.Event_EVENT_UPGRADE_FRPP, Data: data})
	if response == nil || response.Event != pb.Event_EVENT_ERROR || strings.Contains(string(response.Data), "unknown event") {
		t.Fatalf("upgrade was not handled and rejected safely: %v", response)
	}
}
