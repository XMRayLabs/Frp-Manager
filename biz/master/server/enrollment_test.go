package server

import (
	"net/http/httptest"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func TestServerEnrollmentDiscoversAddressAndRetrievesIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.Server{}); err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	manager := models.NewDBManager(defs.DBTypeSQLite3)
	manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(manager)
	ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginCtx.Request = httptest.NewRequest("POST", "/api/v1/server/init", nil)
	ginCtx.Request.RemoteAddr = "192.0.2.44:4000"
	ginCtx.Set(defs.UserInfoKey, &models.UserEntity{UserID: 1, TenantID: 1, UserName: "owner", Role: defs.UserRole_Admin})
	ctx := app.NewContext(ginCtx, instance)
	created, err := InitServerHandler(ctx, &pb.InitServerRequest{ServerId: lo.ToPtr("new-node")})
	if err != nil {
		t.Fatal(err)
	}
	if created.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
		t.Fatal(created)
	}
	found, err := GetServerHandler(ctx, &pb.GetServerRequest{ServerId: lo.ToPtr("new-node")})
	if err != nil {
		t.Fatal(err)
	}
	if found.GetServer().GetId() != created.GetServerId() || found.GetServer().GetSecret() == "" || found.GetServer().GetIp() != "192.0.2.44" {
		t.Fatalf("unexpected discovered server: id=%s ip=%s", found.GetServer().GetId(), found.GetServer().GetIp())
	}
}
