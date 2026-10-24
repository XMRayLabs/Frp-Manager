package client

import (
	"context"
	"testing"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/tunnel"
	"github.com/Sakurame1/frp-manager/utils"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/samber/lo"
	"google.golang.org/grpc"
)

type pullConfigAPI struct {
	pb.MasterClient
	root     string
	response *pb.Client
	calls    map[string]int
}

func (p *pullConfigAPI) PullClientConfig(_ context.Context, req *pb.PullClientConfigReq, _ ...grpc.CallOption) (*pb.PullClientConfigResp, error) {
	id := req.GetBase().GetClientId()
	p.calls[id]++
	if id == p.root {
		return &pb.PullClientConfigResp{Client: p.response}, nil
	}
	return &pb.PullClientConfigResp{Client: &pb.Client{OriginClientId: lo.ToPtr(p.root)}}, nil
}

type pullConfigMaster struct{ api pb.MasterClient }

func (m pullConfigMaster) Call() pb.MasterClient { return m.api }

type pullConfigHandler struct {
	app.ClientHandler
	stops   int
	running bool
}

func (h *pullConfigHandler) Stop()         { h.stops++ }
func (h *pullConfigHandler) Running() bool { return h.running }

func TestPullConfigRemovesExpiredChildrenAndPullsEachChildOnce(t *testing.T) {
	root, active, expired := "admin.c.main", "admin.c.old@1", "admin.c.old@2"
	instance := app.NewApp()
	ctrl := tunnel.NewClientController()
	instance.SetClientController(ctrl)
	old, keep := &pullConfigHandler{}, &pullConfigHandler{}
	ctrl.Add(expired, "default", old)
	ctrl.Add(active, "default", keep)
	api := &pullConfigAPI{root: root, response: &pb.Client{ClientIds: []string{active, active}}, calls: map[string]int{}}
	instance.SetMasterCli(pullConfigMaster{api})
	if err := PullConfig(instance, root, "secret"); err != nil {
		t.Fatal(err)
	}
	if ctrl.Get(expired, "default") != nil || old.stops != 1 {
		t.Fatal("expired child was not stopped and removed")
	}
	if ctrl.Get(active, "default") != keep || keep.stops != 0 {
		t.Fatal("active child was removed")
	}
	if api.calls[active] != 1 {
		t.Fatalf("same child pulled %d times", api.calls[active])
	}
	api.response = &pb.Client{}
	if err := PullConfig(instance, root, "secret"); err != nil {
		t.Fatal(err)
	}
	if ctrl.Get(active, "default") != nil || keep.stops != 1 {
		t.Fatal("last removed child kept running")
	}
}
func TestPullConfigPreservesDirectPhysicalClient(t *testing.T) {
	const root = "admin.c.direct"
	raw := `{"serverAddr":"127.0.0.1","serverPort":7000,"user":"admin","proxies":[{"name":"admin.home.default.8080.tcp","type":"tcp","remotePort":8080}]}`
	instance := app.NewApp()
	ctrl := tunnel.NewClientController()
	instance.SetClientController(ctrl)
	cfg := &pullConfigAPI{root: root, response: &pb.Client{Config: lo.ToPtr(raw), ServerId: lo.ToPtr("default")}, calls: map[string]int{}}
	instance.SetMasterCli(pullConfigMaster{cfg})
	// Load the same common config to avoid starting a real FRP network service.
	common, _, _, err := utils.LoadClientConfig([]byte(raw), true)
	if err != nil {
		t.Fatal(err)
	}
	handler := &directPullHandler{pullConfigHandler: pullConfigHandler{running: true}, common: common}
	ctrl.Add(root, "default", handler)
	if err := PullConfig(instance, root, "secret"); err != nil {
		t.Fatal(err)
	}
	if ctrl.Get(root, "default") != handler || handler.stops != 0 || handler.updates != 1 {
		t.Fatal("direct client incorrectly treated as expired child")
	}
}

type directPullHandler struct {
	pullConfigHandler
	common  *v1.ClientCommonConfig
	updates int
}

func (h *directPullHandler) GetCommonCfg() *v1.ClientCommonConfig                { return h.common }
func (h *directPullHandler) Update([]v1.ProxyConfigurer, []v1.VisitorConfigurer) { h.updates++ }
