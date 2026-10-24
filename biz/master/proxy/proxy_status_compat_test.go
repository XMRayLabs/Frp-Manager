package proxy

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"testing"

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

type proxyStatusStream struct {
	pb.Master_ServerSendServer
	pending   *sync.Map
	prefix    string
	legacy    bool
	mu        sync.Mutex
	requested []string
}

func (s *proxyStatusStream) Send(req *pb.ServerMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	response := &pb.ClientMessage{SessionId: req.SessionId, Event: req.Event}
	lookup := func(clientID, serverID, name string) *pb.ProxyWorkingStatus {
		s.requested = append(s.requested, name)
		if clientID != "owner.c.device@1" || serverID != "default" {
			return proxyStatus("error", "cannot get client")
		}
		switch name {
		case s.prefix + "owner.device.default.65530.tcp":
			return proxyStatus("running", "")
		case s.prefix + "owner.conflict.default.65531.tcp":
			return proxyStatus("start error", "port already used")
		default:
			return proxyStatus("error", "cannot get proxy status")
		}
	}
	switch req.Event {
	case pb.Event_EVENT_GET_PROXY_INFO_BATCH:
		if s.legacy {
			response.Event = pb.Event_EVENT_ERROR
			response.Data = []byte("unknown event")
			break
		}
		var input pb.GetProxyStatusesRPCRequest
		if err := proto.Unmarshal(req.Data, &input); err != nil {
			return err
		}
		output := &pb.GetProxyStatusesRPCResponse{}
		for _, target := range input.Targets {
			output.ProxyStatuses = append(output.ProxyStatuses, proxyStatusResult(target.GetProxyId(), lookup(target.GetClientId(), target.GetServerId(), target.GetName())))
		}
		response.Data, _ = proto.Marshal(output)
	case pb.Event_EVENT_GET_PROXY_INFO:
		var input pb.GetProxyConfigRequest
		if err := proto.Unmarshal(req.Data, &input); err != nil {
			return err
		}
		status := lookup(input.GetClientId(), input.GetServerId(), input.GetName())
		if proxyStatusNameMissing(status) {
			response.Event = pb.Event_EVENT_ERROR
			response.Data = []byte("cannot get proxy status")
		} else {
			response.Data, _ = proto.Marshal(&pb.GetProxyConfigResponse{WorkingStatus: status})
		}
	default:
		return fmt.Errorf("unexpected event %s", req.Event)
	}
	pending, ok := s.pending.Load(req.SessionId)
	if !ok {
		return fmt.Errorf("missing RPC response channel")
	}
	pending.(chan *pb.ClientMessage) <- response
	return nil
}
func (s *proxyStatusStream) Recv() (*pb.ClientMessage, error) { return nil, io.EOF }
func (s *proxyStatusStream) SetHeader(metadata.MD) error      { return nil }
func (s *proxyStatusStream) SendHeader(metadata.MD) error     { return nil }
func (s *proxyStatusStream) SetTrailer(metadata.MD)           {}
func (s *proxyStatusStream) Context() context.Context         { return context.Background() }
func (s *proxyStatusStream) SendMsg(any) error                { return nil }
func (s *proxyStatusStream) RecvMsg(any) error                { return io.EOF }

func TestProxyStatusCompatibilityUsesKernelNamesAndConfiguredOwner(t *testing.T) {
	for _, tc := range []struct {
		version        string
		modern, legacy bool
	}{
		{"1.0.0", true, true}, {"1.0.1", true, false}, {"1.1.0-armv7-test", true, false}, {"v1.1.5", true, false},
		{"1.1.6", true, false}, {"v1.1.8", true, false}, {"1.1.9", true, false},
		{"main", true, false}, {"", true, false}, {"legacy", false, true}, {"legacy", false, false},
	} {
		t.Run(fmt.Sprintf("%s/raw=%v/legacy=%v", tc.version, tc.modern, tc.legacy), func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+url.QueryEscape(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&models.Client{}, &models.Server{}); err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "owner.c.device@1", OriginClientID: "owner.c.device", ServerID: "default", TenantID: 1, UserID: 2, ConfigContent: []byte(`{"user":"owner"}`)}}).Error; err != nil {
				t.Fatal(err)
			}
			// The default server has a stable runtime key in this fixture.
			if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "default", RuntimeID: "default"}}).Error; err != nil {
				t.Fatal(err)
			}
			instance := app.NewApp()
			manager := models.NewDBManager(defs.DBTypeSQLite3)
			manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
			instance.SetDBManager(manager)
			instance.SetClientsManager(rpc.NewClientsManager())
			pending := &sync.Map{}
			instance.SetClientRecvMap(pending)
			stream := &proxyStatusStream{pending: pending, prefix: "owner.", legacy: tc.legacy}
			if tc.modern {
				stream.prefix = ""
			}
			instance.GetClientsManager().Set("owner.c.device", defs.CliTypeClient, stream, &pb.ClientVersion{GitVersion: tc.version})
			// The administrator viewing these proxies is not their owner.
			viewer := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Role: defs.UserRole_Admin}
			ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, viewer), instance)
			configs := []*models.ProxyConfig{}
			for i, name := range []string{"owner.device.default.65530.tcp", "owner.conflict.default.65531.tcp", "missing", "stopped"} {
				cfg := &models.ProxyConfig{Model: &gorm.Model{ID: uint(980000 + i)}, ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: "owner.c.device@1", OriginClientID: "owner.c.device", ServerID: "default", Name: name, Stopped: i == 3}}
				configs = append(configs, cfg)
				proxyStatusCache.Delete(uint32(cfg.ID))
				defer proxyStatusCache.Delete(uint32(cfg.ID))
			}
			results := collectProxyStatuses(ctx, configs)
			statuses := map[uint32]*pb.ProxyWorkingStatus{}
			for _, result := range results {
				statuses[result.GetProxyId()] = result.GetWorkingStatus()
			}
			for i, want := range []string{"running", "start error", "error", "stopped"} {
				if got := statuses[uint32(configs[i].ID)].GetStatus(); got != want {
					t.Fatalf("proxy %d: want %s got %s (%v)", i, want, got, statuses)
				}
			}
			if statuses[uint32(configs[1].ID)].GetErr() != "port already used" {
				t.Fatal("real runtime failure was hidden")
			}
			if !strings.HasSuffix(statuses[uint32(configs[2].ID)].GetErr(), "cannot get proxy status") {
				t.Fatal("missing proxy incorrectly reported as healthy")
			}
			wantRequests := 4
			if !tc.modern {
				wantRequests = 6
			}
			if len(stream.requested) != wantRequests {
				t.Fatalf("unexpected compatibility requests: %v", stream.requested)
			}
			for _, name := range stream.requested {
				if strings.HasPrefix(name, "admin.") {
					t.Fatal("viewer prefix leaked into runtime query", name)
				}
			}
		})
	}
}
