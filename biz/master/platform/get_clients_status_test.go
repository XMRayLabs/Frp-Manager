package platform

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/glebarez/sqlite"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type statusTestStream struct {
	ctx     context.Context
	pending *sync.Map
}

func (s *statusTestStream) Send(req *pb.ServerMessage) error {
	if req.GetEvent() != pb.Event_EVENT_PING {
		return nil
	}
	version, _ := proto.Marshal(&pb.ClientVersion{GitVersion: "1.0.0"})
	value, ok := s.pending.Load(req.GetSessionId())
	if !ok {
		return nil
	}
	value.(chan *pb.ClientMessage) <- &pb.ClientMessage{
		Event:     pb.Event_EVENT_PONG,
		SessionId: req.GetSessionId(),
		Data:      version,
	}
	return nil
}
func (s *statusTestStream) Recv() (*pb.ClientMessage, error) { return nil, io.EOF }
func (s *statusTestStream) SetHeader(metadata.MD) error      { return nil }
func (s *statusTestStream) SendHeader(metadata.MD) error     { return nil }
func (s *statusTestStream) SetTrailer(metadata.MD)           {}
func (s *statusTestStream) Context() context.Context         { return s.ctx }
func (s *statusTestStream) SendMsg(any) error                { return nil }
func (s *statusTestStream) RecvMsg(any) error                { return io.EOF }

func TestGetClientsStatusReturns300NodesWithoutWaitingForRPC(t *testing.T) {
	const clientCount = 300

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}

	clients := make([]*models.Client, 0, clientCount)
	clientIDs := make([]string, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		clientID := fmt.Sprintf("client-%03d", i)
		clientIDs = append(clientIDs, clientID)
		clients = append(clients, &models.Client{ClientEntity: &models.ClientEntity{
			ClientID:      clientID,
			TenantID:      1,
			UserID:        1,
			ConnectSecret: fmt.Sprintf("secret-%03d", i),
		}})
	}
	if err := db.CreateInBatches(clients, 100).Error; err != nil {
		t.Fatal(err)
	}

	appInstance := app.NewApp()
	dbManager := models.NewDBManager(defs.DBTypeSQLite3)
	dbManager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	appInstance.SetDBManager(dbManager)
	pending := &sync.Map{}
	appInstance.SetClientRecvMap(pending)
	appInstance.SetClientsManager(rpc.NewClientsManager())

	version := &pb.ClientVersion{GitVersion: "1.0.0"}
	for _, clientID := range clientIDs {
		appInstance.GetClientsManager().Set(
			clientID,
			defs.CliTypeClient,
			&statusTestStream{ctx: context.Background(), pending: pending},
			version,
		)
	}

	user := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}
	requestContext := context.WithValue(context.Background(), defs.UserInfoKey, user)
	startedAt := time.Now()
	resp, err := GetClientsStatus(app.NewContext(requestContext, appInstance), &pb.GetClientsStatusRequest{
		ClientType: pb.ClientType_CLIENT_TYPE_FRPC,
		ClientIds:  clientIDs,
	})
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetClients()) != clientCount {
		t.Fatalf("expected %d statuses, got %d", clientCount, len(resp.GetClients()))
	}
	if elapsed > 2*time.Second {
		t.Fatalf("status lookup took too long: %s", elapsed)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		completed := 0
		for _, clientID := range clientIDs {
			if snapshot, ok := appInstance.GetClientsManager().GetRuntimeSnapshot(clientID); ok && !snapshot.CheckedAt.IsZero() {
				completed++
			}
		}
		if completed == clientCount {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d background probes completed", completed, clientCount)
		}
		for _, id := range clientIDs {
			scheduleStatusProbe(appInstance, id, appInstance.GetClientsManager().Get(id))
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Logf("returned %d client statuses in %s before background probes completed", clientCount, elapsed)
}

func TestSaturatedProbesDoNotBlockOrMarkNodesPending(t *testing.T) {
	for len(statusProbeSlots) > 0 {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < cap(statusProbeSlots); i++ {
		statusProbeSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(statusProbeSlots); i++ {
			<-statusProbeSlots
		}
	}()
	instance := app.NewApp()
	instance.SetClientsManager(rpc.NewClientsManager())
	instance.GetClientsManager().Set("busy-node", defs.CliTypeClient, &statusTestStream{ctx: context.Background(), pending: &sync.Map{}}, &pb.ClientVersion{})
	connector := instance.GetClientsManager().Get("busy-node")
	scheduleStatusProbe(instance, "busy-node", connector)
	if !instance.GetClientsManager().TryStartStatusProbe("busy-node", 0) {
		t.Fatal("saturated probe marked node in flight")
	}
}
