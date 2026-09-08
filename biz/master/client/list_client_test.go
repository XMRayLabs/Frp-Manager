package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type queryCountLogger struct {
	gormlogger.Interface
	count atomic.Int64
}

func (l *queryCountLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (string, int64),
	err error,
) {
	l.count.Add(1)
	l.Interface.Trace(ctx, begin, fc, err)
}

func TestListClientsUsesBoundedQueries(t *testing.T) {
	counter := &queryCountLogger{
		Interface: gormlogger.New(log.New(io.Discard, "", 0), gormlogger.Config{LogLevel: gormlogger.Silent}),
	}
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: counter})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}

	const clientCount = 300
	clients := make([]*models.Client, 0, clientCount+1)
	for i := 0; i < clientCount; i++ {
		clients = append(clients, &models.Client{ClientEntity: &models.ClientEntity{
			ClientID:      fmt.Sprintf("client-%03d", i),
			TenantID:      1,
			UserID:        1,
			ConnectSecret: fmt.Sprintf("secret-%03d", i),
		}})
	}
	clients = append(clients, &models.Client{ClientEntity: &models.ClientEntity{
		ClientID:       "shadow-client",
		OriginClientID: "client-000",
		TenantID:       1,
		UserID:         1,
		ConnectSecret:  "shadow-secret",
	}})
	if err := db.CreateInBatches(clients, 100).Error; err != nil {
		t.Fatal(err)
	}

	appInstance := app.NewApp()
	dbManager := models.NewDBManager(defs.DBTypeSQLite3)
	dbManager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	appInstance.SetDBManager(dbManager)

	user := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}
	requestContext := context.WithValue(context.Background(), defs.UserInfoKey, user)
	counter.count.Store(0)
	page := int32(1)
	pageSize := int32(clientCount)
	resp, err := ListClientsHandler(app.NewContext(requestContext, appInstance), &pb.ListClientsRequest{
		Page:     &page,
		PageSize: &pageSize,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetClients()) != clientCount {
		t.Fatalf("expected %d clients, got %d", clientCount, len(resp.GetClients()))
	}
	if got := resp.GetClients()[0].GetClientIds(); len(got) != 1 || got[0] != "shadow-client" {
		t.Fatalf("unexpected shadow clients: %v", got)
	}
	if queries := counter.count.Load(); queries > 4 {
		t.Fatalf("expected at most 4 SQL queries, got %d", queries)
	} else {
		t.Logf("listed %d clients with %d SQL queries", clientCount, queries)
	}
}
