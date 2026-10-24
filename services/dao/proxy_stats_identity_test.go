package dao

import (
	"fmt"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/samber/lo"
)

func TestProxyStatsMatchConfiguredWireNameAndOwner(t *testing.T) {
	db, ctx, admin := websiteTestDB(t, false)
	if err := db.AutoMigrate(&models.ProxyStats{}); err != nil {
		t.Fatal(err)
	}
	srv := &models.ServerEntity{ServerID: defs.DefaultServerID}
	type row struct {
		client, name, user string
		owner, tenant      int
	}
	cases := []row{
		{"admin.c.home@1", "admin.home.default.60001.tcp", "admin", 1, 1},
		{"alice.c.home@1", "shared.name", "alice", 2, 1},
		{"bob.c.home@1", "shared.name", "bob", 3, 1},
		{"alice.c.other@1", "alice.other.default.60004.tcp", "alice", 2, 1},
		{"tenant.c.home@1", "tenant.home.default.60005.tcp", "tenant", 4, 2},
		{"legacy.c.home@1", "admin.raw.name", "", 1, 1},
	}
	for _, r := range cases {
		cfg := fmt.Sprintf(`{"user":%q,"proxies":[{"name":%q,"type":"tcp","remotePort":60001}]}`, r.user, r.name)
		node := &models.ClientEntity{ClientID: r.client, OriginClientID: r.client + "-physical", ServerID: srv.ServerID, UserID: r.owner, TenantID: r.tenant, ConfigContent: []byte(cfg)}
		if err := db.Create(&models.Client{ClientEntity: node}).Error; err != nil {
			t.Fatal(err)
		}
	}
	old := &models.ProxyStats{ProxyStatsEntity: &models.ProxyStatsEntity{ServerID: srv.ServerID, ClientID: cases[1].client, OriginClientID: cases[1].client + "-physical", Name: cases[1].name, UserID: 2, TenantID: 1, TodayTrafficIn: 10, TodayTrafficOut: 20, HistoryTrafficIn: 100, HistoryTrafficOut: 200, UpdatedAt: time.Now().Add(-48 * time.Hour)}}
	if err := db.Create(old).Error; err != nil {
		t.Fatal(err)
	}
	inputs := []*pb.ProxyInfo{nil}
	for i, r := range cases {
		wireName := r.name
		if r.user != "" {
			wireName = r.user + "." + r.name
		}
		inputs = append(inputs, &pb.ProxyInfo{Name: lo.ToPtr(wireName), Type: lo.ToPtr("tcp"), TodayTrafficIn: lo.ToPtr(int64(30 + i)), TodayTrafficOut: lo.ToPtr(int64(60 + i))})
	}
	inputs = append(inputs, &pb.ProxyInfo{Name: lo.ToPtr("admin.not-configured"), Type: lo.ToPtr("tcp"), TodayTrafficIn: lo.ToPtr(int64(999))})
	mutation := NewMutation(ctx)
	if err := mutation.AdminUpdateProxyStats(srv, inputs); err != nil {
		t.Fatal(err)
	}
	if err := mutation.AdminUpdateProxyStats(srv, inputs); err != nil {
		t.Fatal(err)
	}
	var rows []models.ProxyStats
	if err := db.Order("client_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(cases) {
		t.Fatalf("missing or duplicated stats: %+v", rows)
	}
	for _, r := range cases {
		var got models.ProxyStats
		if err := db.Where("client_id = ? AND name = ?", r.client, r.name).First(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.UserID != r.owner || got.TenantID != r.tenant {
			t.Fatalf("wrong owner for %s: %+v", r.client, got.ProxyStatsEntity)
		}
		if r.client == cases[1].client && (got.ProxyID != old.ProxyID || got.HistoryTrafficIn != 110 || got.HistoryTrafficOut != 220) {
			t.Fatalf("lost or double-counted historical traffic: %+v", got.ProxyStatsEntity)
		}
	}
	stats, err := NewQuery(ctx).GetProxyStatsByClientID(admin, cases[1].client)
	if err != nil || len(stats) != 1 || stats[0].UserID != 2 {
		t.Fatalf("admin cannot view actual owner stats: %+v %v", stats, err)
	}
	outsider := &models.UserEntity{UserID: 9, TenantID: 1, UserName: "outsider", Role: defs.UserRole_Normal, Status: models.STATUS_NORMAL}
	if _, err := NewQuery(ctx).GetProxyStatsByClientID(outsider, cases[1].client); err == nil {
		t.Fatal("unauthorized stats access")
	}
	// A separate server with the same proxy name must not reuse this history.
	other := &models.ServerEntity{ServerID: "admin.s.other", TenantID: 1, UserID: 1}
	cfg := fmt.Sprintf(`{"user":"alice","proxies":[{"name":%q,"type":"tcp","remotePort":60001}]}`, cases[1].name)
	if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "alice.c.separate", ServerID: other.ServerID, UserID: 2, TenantID: 1, ConfigContent: []byte(cfg)}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := mutation.AdminUpdateProxyStats(other, []*pb.ProxyInfo{inputs[2]}); err != nil {
		t.Fatal(err)
	}
	var got models.ProxyStats
	if err := db.Where("server_id = ?", other.ServerID).First(&got).Error; err != nil || got.HistoryTrafficIn != 0 {
		t.Fatal("history crossed servers", got, err)
	}
}
