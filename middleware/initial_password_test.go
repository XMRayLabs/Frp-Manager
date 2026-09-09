package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestInitialPasswordGateCannotBeBypassedByDirectAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:password-gate-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	account := &models.UserEntity{UserID: 1, UserName: "ig01", Email: "ig01@test.de", MustChangePassword: true}
	if err = db.Create(&models.User{UserEntity: account}).Error; err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(mgr)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(defs.UserIDKey, 1); c.Next() }, AuthCtx(instance))
	for _, path := range []string{"/api/v1/user/change-initial-password", "/api/v1/user/password-status", "/api/v1/user/update", "/api/v1/client/init", "/api/v1/user/enrollment-token"} {
		router.POST(path, func(c *gin.Context) { c.String(http.StatusOK, "allowed") })
	}
	call := func(path string) int {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("POST", path, nil))
		return rec.Code
	}
	for _, path := range []string{"/api/v1/user/update", "/api/v1/client/init", "/api/v1/user/enrollment-token"} {
		if call(path) != http.StatusForbidden {
			t.Fatalf("bypassed first-password gate: %s", path)
		}
	}
	if call("/api/v1/user/change-initial-password") != http.StatusOK || call("/api/v1/user/password-status") != http.StatusOK {
		t.Fatal("password change flow blocked")
	}
	db.Model(&models.User{}).Where("user_id = ?", 1).Update("must_change_password", false)
	if call("/api/v1/client/init") != http.StatusOK {
		t.Fatal("account remains blocked after password change")
	}
}
