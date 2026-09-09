package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"gorm.io/gorm"
)

func TestOverviewPreservesCachedFailureDuringRefresh(t *testing.T) {
	const id = 987654
	// A fresh cache entry needs no connected node or RPC infrastructure.
	storeProxyStatus(id, proxyStatus("start error", "port already in use"))
	defer proxyStatusCache.Delete(uint32(id))
	cfg := &models.ProxyConfig{Model: &gorm.Model{ID: id}, ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: "child", OriginClientID: "physical"}}
	ctx := app.NewContext(context.Background(), app.NewApp())
	start := time.Now()
	result := OverviewProxyStatuses(ctx, map[string]string{"child": "owner"}, []*models.ProxyConfig{cfg})
	if len(result) != 1 || result[0].GetWorkingStatus().GetErr() != "port already in use" {
		t.Fatalf("lost runtime failure: %v", result)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cached status blocked")
	}
}

func TestProxyRuntimeNameUsesConfiguredOwner(t *testing.T) {
	if proxyRuntimeName("owner", "web") != "owner.web" {
		t.Fatal("owner prefix missing")
	}
	if proxyRuntimeName("", "web") != "web" {
		t.Fatal("empty owner adds a dot")
	}
}
