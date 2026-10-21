package permission

import (
	"bytes"
	"encoding/json"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"testing"
)

func TestServerPortAssignmentEditingAndPermissions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.User{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.Server{}, &models.ProxyConfig{}, &models.OrganizationAudit{}); err != nil {
		t.Fatal(err)
	}
	admin := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin, Status: 1}
	member := &models.UserEntity{UserID: 2, TenantID: 1, UserName: "member", Email: "member", LanguageGroupID: "g", Role: defs.UserRole_Normal, Status: 1}
	for _, obj := range []interface{}{&models.User{UserEntity: member}, &models.LanguageGroup{ID: "g", TenantID: 1, Name: "G"}, &models.Server{ServerEntity: &models.ServerEntity{ServerID: "s", TenantID: 1}}} {
		if err := db.Create(obj).Error; err != nil {
			t.Fatal(err)
		}
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	var actor models.UserInfo = admin
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(defs.UserInfoKey, actor); c.Next() })
	OrganizationRoutes(router.Group("/org"), a)
	call := func(ports *string) *httptest.ResponseRecorder {
		body := map[string]interface{}{"id": "g", "server_id": "s", "assigned": true}
		if ports != nil {
			body["allowed_ports"] = *ports
		}
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/org/servers/assign", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	assertGrant := func(want string) {
		t.Helper()
		var g models.LanguageGroupServer
		if err := db.First(&g).Error; err != nil || g.AllowedPorts != want {
			t.Fatalf("grant=%+v err=%v", g, err)
		}
	}
	ports := "62000-63999"
	if w := call(&ports); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	assertGrant(ports)
	if w := call(nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	assertGrant(ports)
	original := []byte(`{"type":"tcp","remotePort":62000,"plugin":{"type":"socks5","username":"u","password":"p"}}`)
	proxy := &models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{UserID: 2, ServerID: "s", ClientID: "c", Name: "existing", Content: original, Stopped: true}}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatal(err)
	}
	ports = "63000-63999"
	if w := call(&ports); w.Code != 400 || !bytes.Contains(w.Body.Bytes(), []byte("existing")) {
		t.Fatal(w.Code, w.Body.String())
	}
	assertGrant("62000-63999")
	var saved models.ProxyConfig
	db.First(&saved)
	if !saved.Stopped || !bytes.Equal(saved.Content, original) {
		t.Fatal("rejected edit changed proxy")
	}
	if err := validateMovingUserPorts(db, 2, "g"); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.LanguageGroupServer{}).Where("language_group_id = ?", "g").Update("allowed_ports", "63000-63999").Error; err != nil {
		t.Fatal(err)
	}
	if err := validateMovingUserPorts(db, 2, "g"); err == nil {
		t.Fatal("migration bypassed destination ports")
	}
	db.Model(&models.LanguageGroupServer{}).Where("language_group_id = ?", "g").Update("allowed_ports", "62000-63999")
	ports = "invalid"
	if w := call(&ports); w.Code != 400 {
		t.Fatal(w.Code)
	}
	assertGrant("62000-63999")
	ports = ""
	if w := call(&ports); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	assertGrant("")
	actor = &models.UserEntity{UserID: 3, TenantID: 1, Role: defs.UserRole_GroupAdmin, LanguageGroupID: "g", Status: 1}
	ports = "62000-63999"
	if w := call(&ports); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	assertGrant("")
}
