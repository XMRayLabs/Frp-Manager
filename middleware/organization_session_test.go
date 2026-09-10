package middleware_test

import (
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/middleware"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
)

func TestSessionVersionRejectsOldTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:org-session?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.User{UserEntity: &models.UserEntity{UserID: 1, UserName: "user", Email: "user", Status: 1}})
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	cfg := conf.Config{}
	cfg.App.GlobalSecret = "test-only-session-secret"
	cfg.App.CookieAge = 3600
	cfg.App.CookieName = "session-test"
	a.SetConfig(cfg)
	router := gin.New()
	router.Use(middleware.JWTAuth(a), middleware.AuthCtx(a))
	router.GET("/check", func(c *gin.Context) { c.String(200, "ok") })
	call := func(token string) int {
		r := httptest.NewRequest("GET", "/check", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}
	old := conf.GetJWTWithAllPermission(cfg, 1)
	if call(old) != 200 {
		t.Fatal("legacy session rejected before invalidation")
	}
	db.Model(&models.User{}).Where("user_id = ?", 1).Update("session_version", 1)
	if call(old) != 401 {
		t.Fatal("old session accepted")
	}
	fresh, err := conf.GetJWTWithPayload(cfg, 1, map[string]interface{}{"session_version": 1})
	if err != nil {
		t.Fatal(err)
	}
	if call(fresh) != 200 {
		t.Fatal("new session rejected")
	}
	db.Model(&models.User{}).Where("user_id = ?", 1).Update("status", models.STATUS_BANED)
	if call(fresh) != 401 {
		t.Fatal("disabled account accepted")
	}
}
