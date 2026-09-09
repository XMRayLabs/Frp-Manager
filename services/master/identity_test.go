package master

import (
	"context"
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	"net"
	"sync"
	"testing"
	"time"
)

func TestAuthenticatedStreamSurvivesRenameAndNameReuse(t *testing.T) {
	for _, serverRole := range []bool{false, true} {
		t.Run(fmt.Sprint(serverRole), func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err = db.AutoMigrate(&models.Client{}, &models.Server{}); err != nil {
				t.Fatal(err)
			}
			a := app.NewApp()
			dbm := models.NewDBManager(defs.DBTypeSQLite3)
			dbm.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
			a.SetDBManager(dbm)
			a.SetClientsManager(rpc.NewClientsManager())
			a.SetClientRecvMap(&sync.Map{})
			add := func(id, secret string) {
				t.Helper()
				var row interface{} = &models.Client{ClientEntity: &models.ClientEntity{ClientID: id, ConnectSecret: secret, TenantID: 1, UserID: 1}}
				if serverRole {
					row = &models.Server{ServerEntity: &models.ServerEntity{ServerID: id, ConnectSecret: secret, TenantID: 1, UserID: 1}}
				}
				if err := db.Create(row).Error; err != nil {
					t.Fatal(err)
				}
			}
			add("old", "original-secret")
			listener := bufconn.Listen(1024 * 1024)
			g := grpc.NewServer()
			pb.RegisterMasterServer(g, &server{appInstance: a})
			go g.Serve(listener)
			defer g.Stop()
			conn, err := grpc.NewClient("passthrough:///identity-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			event := pb.Event_EVENT_REGISTER_CLIENT
			if serverRole {
				event = pb.Event_EVENT_REGISTER_SERVER
			}
			connect := func(secret, want string) pb.Master_ServerSendClient {
				t.Helper()
				stream, err := pb.NewMasterClient(conn).ServerSend(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if err = stream.Send(&pb.ClientMessage{Event: event, ClientId: "old", Secret: secret}); err != nil {
					t.Fatal(err)
				}
				ack, err := stream.Recv()
				if err != nil || ack.GetClientId() != want {
					t.Fatalf("registration: %v %v", ack, err)
				}
				return stream
			}
			original := connect("original-secret", "old")
			table, column := "clients", "client_id"
			if serverRole {
				table, column = "servers", "server_id"
			}
			models.NodeIdentityMu.Lock()
			err = db.Table(table).Where(column+" = ?", "old").Update(column, "renamed").Error
			a.GetClientsManager().Rename("old", "renamed")
			models.NodeIdentityMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			add("old", "new-secret")
			reused := connect("new-secret", "old")
			callDone := make(chan error, 1)
			go func() {
				_, err := rpc.CallClient(app.NewContext(ctx, a), "renamed", pb.Event_EVENT_PING, &pb.CommonRequest{})
				callDone <- err
			}()
			message, err := original.Recv()
			if err != nil {
				t.Fatal(err)
			}
			if err = original.Send(&pb.ClientMessage{SessionId: message.SessionId, Event: pb.Event_EVENT_DATA}); err != nil {
				t.Fatal(err)
			}
			if err = <-callDone; err != nil {
				t.Fatal("renamed stream call failed", err)
			}
			restarted := connect("original-secret", "renamed")
			if _, err = original.Recv(); err == nil {
				t.Fatal("replaced stream remained open")
			}
			if a.GetClientsManager().Get("old") == nil {
				t.Fatal("old stream cleanup removed new device")
			}
			models.NodeIdentityMu.Lock()
			err = db.Table(table).Where(column+" = ?", "renamed").Update("connect_secret", "revoked").Error
			a.GetClientsManager().Remove("renamed")
			models.NodeIdentityMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = restarted.Recv(); err == nil {
				t.Fatal("revoked stream remained open")
			}
			bad, err := pb.NewMasterClient(conn).ServerSend(ctx)
			if err != nil {
				t.Fatal(err)
			}
			bad.Send(&pb.ClientMessage{Event: event, ClientId: "old", Secret: "original-secret"})
			ack, err := bad.Recv()
			if err == nil && ack.GetClientId() != "" {
				t.Fatal("revoked secret authenticated as reused name")
			}
			reused.CloseSend()
		})
	}
}
