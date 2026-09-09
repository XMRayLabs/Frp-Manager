package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	userapi "github.com/Sakurame1/frp-manager/biz/master/user"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/middleware"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFixedEnrollmentTokenScopesAndConfirmation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:enrollment-auth-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.EnrollmentToken{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	owner := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Email: "admin@test", Role: defs.UserRole_Admin}
	if err = db.Create(&models.User{UserEntity: owner}).Error; err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	manager := models.NewDBManager(defs.DBTypeSQLite3)
	manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(manager)
	token, err := models.GetEnrollmentToken(db, 1, "client")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(middleware.JWTAuth(instance), middleware.AuthCtx(instance), middleware.RBAC(instance))
	for _, path := range []string{"/api/v1/client/init", "/api/v1/client/get", "/api/v1/server/init", "/api/v1/user/enrollment-token", "/api/v1/user/enrollment-token/rotate"} {
		router.POST(path, func(c *gin.Context) { c.String(http.StatusOK, "allowed") })
	}
	request := func(path, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set(defs.AuthorizationKey, credential)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	if request("/api/v1/client/init", token).Body.String() != "allowed" {
		t.Fatal("client enrollment denied")
	}
	if request("/api/v1/client/get", "Bearer "+token).Body.String() != "allowed" {
		t.Fatal("bearer enrollment denied")
	}
	for _, path := range []string{"/api/v1/server/init", "/api/v1/user/enrollment-token", "/api/v1/user/enrollment-token/rotate"} {
		if request(path, token).Body.String() == "allowed" {
			t.Fatalf("enrollment token escaped scope: %s", path)
		}
	}
	// Exercise the management handler as an authenticated owner: an absent
	// second confirmation must not revoke the existing credential.
	management := gin.New()
	management.Use(func(c *gin.Context) { c.Set(defs.UserInfoKey, owner); c.Next() })
	management.POST("/rotate", userapi.EnrollmentTokenHandler(instance, true))
	req := httptest.NewRequest("POST", "/rotate", bytes.NewBufferString(`{"role":"client","currentToken":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	management.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatal("rotation without confirmation accepted")
	}
	if _, err = models.ResolveEnrollmentToken(db, token); err != nil {
		t.Fatal("rejected rotation changed token")
	}
	fresh, err := models.RotateEnrollmentToken(db, 1, "client", token)
	if err != nil {
		t.Fatal(err)
	}
	if request("/api/v1/client/init", token).Code != http.StatusUnauthorized {
		t.Fatal("old credential accepted after rotation")
	}
	if request("/api/v1/client/init", fresh).Body.String() != "allowed" {
		t.Fatal("new credential rejected")
	}
}
