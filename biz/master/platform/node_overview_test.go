package platform

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOverviewReasonsDeduplicate(t *testing.T) {
	var stats nodeOverview
	stats.add(true, true, true, true, true)
	stats.add(true, false, false, false, false)
	if stats.Total != 2 || stats.Online != 2 || stats.Pending != 1 || stats.Upgrade != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestNodeUpgradeVersions(t *testing.T) {
	for _, tc := range []struct {
		old, current string
		want         bool
	}{
		{"v1.0.0", "1.0.1", true}, {"1.0.1", "v1.0.1", false},
		{"1.1.0", "1.0.1", false}, {"1.0.1-rc.1", "1.0.1", true},
		{"", "1.0.1", false}, {"dev", "1.0.1", false},
	} {
		if got := needsNodeUpgrade(tc.old, tc.current); got != tc.want {
			t.Errorf("%s -> %s: %v", tc.old, tc.current, got)
		}
	}
}

func TestServerIncompleteAndInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		cfg, ip string
		bad     bool
	}{
		{"", "", true}, {"{", "host", true}, {`{"bindPort":7000}`, "", true},
		{`{"bindPort":-1}`, "host", true}, {`{"bindPort":7000}`, "host", false},
	} {
		if got := serverConfigInvalid(&models.ServerEntity{ConfigContent: []byte(tc.cfg), ServerIP: tc.ip}); got != tc.bad {
			t.Errorf("%s %s: %v", tc.cfg, tc.ip, got)
		}
	}
}

func TestOverviewUsesVisiblePhysicalNodesAndDoesNotWaitForRPC(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:overview-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Client{}, &models.Server{}, &models.ProxyConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []*models.ClientEntity{
		{ClientID: "new", TenantID: 1, UserID: 1, ConnectSecret: "a"},
		{ClientID: "partial", TenantID: 1, UserID: 1, ConnectSecret: "b", ConfigContent: []byte(`{"serverAddr":"example.com","serverPort":7000}`)},
		{ClientID: "paused", TenantID: 1, UserID: 1, ConnectSecret: "c", Stopped: true, ConfigContent: []byte(`{"serverAddr":"example.com","serverPort":7000,"proxies":[{"name":"web","type":"tcp","localPort":80,"remotePort":8080}]}`)},
		{ClientID: "hidden", TenantID: 2, UserID: 2, ConnectSecret: "d"},
		{ClientID: "child", OriginClientID: "partial", TenantID: 1, UserID: 1, ConnectSecret: "e"},
	} {
		if err = db.Create(&models.Client{ClientEntity: row}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []*models.ServerEntity{
		{ServerID: "s1", TenantID: 1, UserID: 1, ConnectSecret: "s", ServerIP: "example.com", ConfigContent: []byte(`{"bindPort":7000}`)},
		{ServerID: "s2", TenantID: 2, UserID: 2, ConnectSecret: "hidden"},
	} {
		if err = db.Create(&models.Server{ServerEntity: row}).Error; err != nil {
			t.Fatal(err)
		}
	}
	instance := app.NewApp()
	manager := models.NewDBManager(defs.DBTypeSQLite3)
	manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(manager)
	instance.SetClientsManager(rpc.NewClientsManager())
	pending := &sync.Map{}
	instance.SetClientRecvMap(pending)
	instance.GetClientsManager().Set("new", defs.CliTypeClient, &statusTestStream{ctx: context.Background(), pending: pending}, &pb.ClientVersion{GitVersion: "dev"})
	user := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, user), instance)
	start := time.Now()
	result, err := getNodeOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("overview blocked on runtime probes")
	}
	if result.Clients.Total != 3 || result.Clients.Online != 1 || result.Clients.Pending != 2 || result.Clients.Unconfigured != 2 {
		t.Fatalf("clients: %+v", result.Clients)
	}
	if result.Servers.Total != 1 || result.Servers.Online != 0 || result.Servers.Pending != 1 {
		t.Fatalf("servers: %+v", result.Servers)
	}
}
