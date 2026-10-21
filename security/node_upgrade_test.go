package security_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	clientapi "github.com/Sakurame1/frp-manager/biz/master/client"
	serverapi "github.com/Sakurame1/frp-manager/biz/master/server"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/samber/lo"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type upgradeStream struct {
	a        app.Application
	requests []*pb.UpgradeFrppRequest
	code     pb.RespCode
	unknown  bool
}

func (f *upgradeStream) Send(m *pb.ServerMessage) error {
	if m.Event != pb.Event_EVENT_UPGRADE_FRPP {
		return fmt.Errorf("wrong event: %v", m.Event)
	}
	req := &pb.UpgradeFrppRequest{}
	if err := proto.Unmarshal(m.Data, req); err != nil {
		return err
	}
	f.requests = append(f.requests, req)
	data, _ := proto.Marshal(&pb.UpgradeFrppResponse{Status: &pb.Status{Code: f.code, Message: "device result"}})
	event := pb.Event_EVENT_DATA
	if f.unknown {
		event = pb.Event_EVENT_ERROR
		data = []byte("unknown event")
	}
	if ch, ok := f.a.GetClientRecvMap().Load(m.SessionId); ok {
		ch.(chan *pb.ClientMessage) <- &pb.ClientMessage{Event: event, Data: data}
	}
	return nil
}
func (*upgradeStream) Recv() (*pb.ClientMessage, error) { return nil, io.EOF }
func (*upgradeStream) SetHeader(metadata.MD) error      { return nil }
func (*upgradeStream) SendHeader(metadata.MD) error     { return nil }
func (*upgradeStream) SetTrailer(metadata.MD)           {}
func (*upgradeStream) Context() context.Context         { return context.Background() }
func (*upgradeStream) SendMsg(any) error                { return nil }
func (*upgradeStream) RecvMsg(any) error                { return io.EOF }

func TestRemoteUpgradeAuthorizationAndAcknowledgement(t *testing.T) {
	a, db, owner := fixture(t)
	addClient(t, db, "owner.c.node", owner.UserID)
	addClient(t, db, "foreign.c.node", 999)
	if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "admin.s.node", TenantID: 1, UserID: 99, ConnectSecret: "server-key"}}).Error; err != nil {
		t.Fatal(err)
	}
	client := &upgradeStream{a: a, code: pb.RespCode_RESP_CODE_SUCCESS}
	server := &upgradeStream{a: a, code: pb.RespCode_RESP_CODE_SUCCESS}
	a.GetClientsManager().Set("owner.c.node", defs.CliTypeClient, client, nil)
	a.GetClientsManager().Set("admin.s.node", defs.CliTypeServer, server, nil)
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, owner), a)
	req := func(ids ...string) *pb.UpgradeFrppRequest {
		return &pb.UpgradeFrppRequest{ClientIds: ids, Version: lo.ToPtr("1.1.4")}
	}
	if _, err := clientapi.UpgradeFrppHandler(ctx, req("owner.c.node", "foreign.c.node")); err == nil {
		t.Fatal("mixed unauthorized batch accepted")
	}
	if len(client.requests) != 0 {
		t.Fatal("partial dispatch before full authorization")
	}
	if _, err := serverapi.UpgradeFrppHandler(ctx, req("admin.s.node")); err == nil {
		t.Fatal("ordinary user upgraded server")
	}
	if _, err := clientapi.UpgradeFrppHandler(ctx, req("owner.c.node")); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 || client.requests[0].GetVersion() != "v1.1.4" || !client.requests[0].GetRestartService() || !client.requests[0].GetBackup() || client.requests[0].GetServiceName() != "frpp" {
		t.Fatalf("unsafe defaults: %v", client.requests)
	}
	malicious := req("owner.c.node")
	malicious.DownloadUrl = lo.ToPtr("https://example.invalid/program")
	if _, err := clientapi.UpgradeFrppHandler(ctx, malicious); err == nil {
		t.Fatal("arbitrary binary accepted")
	}
	badPath := req("owner.c.node")
	badPath.TargetPath = lo.ToPtr("/etc/cron.d/payload")
	if _, err := clientapi.UpgradeFrppHandler(ctx, badPath); err == nil {
		t.Fatal("arbitrary path accepted")
	}
	client.code = pb.RespCode_RESP_CODE_INVALID
	if _, err := clientapi.UpgradeFrppHandler(ctx, req("owner.c.node")); err == nil {
		t.Fatal("negative acknowledgement reported as success")
	}
	a.GetClientsManager().Remove("owner.c.node")
	if _, err := clientapi.UpgradeFrppHandler(ctx, req("owner.c.node")); err == nil {
		t.Fatal("offline client accepted")
	}
	owner.Role = defs.UserRole_Admin
	if _, err := serverapi.UpgradeFrppHandler(ctx, req("admin.s.node")); err != nil {
		t.Fatal(err)
	}
	server.unknown = true
	if _, err := serverapi.UpgradeFrppHandler(ctx, req("admin.s.node")); err == nil || !strings.Contains(err.Error(), "手动升级") {
		t.Fatal("legacy server error not explained", err)
	}
	if _, err := serverapi.UpgradeFrppHandler(ctx, req(defs.DefaultServerID)); err == nil {
		t.Fatal("embedded master upgrade accepted")
	}
	if _, err := clientapi.UpgradeFrppHandler(app.NewContext(context.Background(), a), req("foreign.c.node")); err == nil {
		t.Fatal("anonymous accepted")
	}
}

func TestSharedMembersCannotUpgradeAnotherOwnersClient(t *testing.T) {
	a, db, u := fixture(t)
	owner := &models.UserEntity{UserID: 2, TenantID: 1, UserName: "owner2", Email: "owner2@example.test", Role: defs.UserRole_Normal, LanguageGroupID: "group-a"}
	if err := db.Create(&models.User{UserEntity: owner}).Error; err != nil {
		t.Fatal(err)
	}
	addClient(t, db, "owner2.c.shared", owner.UserID)
	db.Model(&models.LanguageGroup{}).Where("id = ?", "group-a").Update("shared", true)
	stream := &upgradeStream{a: a, code: pb.RespCode_RESP_CODE_SUCCESS}
	a.GetClientsManager().Set("owner2.c.shared", defs.CliTypeClient, stream, nil)
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, u), a)
	req := &pb.UpgradeFrppRequest{ClientIds: []string{"owner2.c.shared"}}
	if _, err := clientapi.UpgradeFrppHandler(ctx, req); err == nil {
		t.Fatal("shared member upgraded another owner's client")
	}
	u.Role = defs.UserRole_GroupAdmin
	if _, err := clientapi.UpgradeFrppHandler(ctx, req); err != nil {
		t.Fatal("same-group administrator denied", err)
	}
	db.Model(&models.User{}).Where("user_id = ?", owner.UserID).Update("language_group_id", "group-b")
	before := len(stream.requests)
	if _, err := clientapi.UpgradeFrppHandler(ctx, req); err == nil {
		t.Fatal("cross-group administrator accepted")
	}
	if len(stream.requests) != before {
		t.Fatal("cross-group upgrade sent")
	}
}
