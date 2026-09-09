package server

import (
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestCoreHealthPingDetectsMissingCoreWithoutBreakingLegacyPing(t *testing.T) {
	instance := app.NewApp()
	legacy := HandleServerMessage(instance, &pb.ServerMessage{Event: pb.Event_EVENT_PING})
	if legacy.GetEvent() != pb.Event_EVENT_PONG {
		t.Fatal("legacy ping must return version")
	}
	marker := "core-health"
	data, err := proto.Marshal(&pb.CommonRequest{Data: &marker})
	if err != nil {
		t.Fatal(err)
	}
	health := HandleServerMessage(instance, &pb.ServerMessage{Event: pb.Event_EVENT_PING, Data: data})
	if health.GetEvent() != pb.Event_EVENT_ERROR {
		t.Fatalf("missing core reported healthy: %v", health)
	}
}
