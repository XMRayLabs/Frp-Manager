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

func TestOrganizationMovePreviewAndMembership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:org-move?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.UserGroup{}, &models.InviteCode{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.OrganizationAudit{}, &models.ProxyConfig{}, &models.Client{}, &models.Server{}); err != nil {
		t.Fatal(err)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	admin := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Email: "admin", Role: defs.UserRole_Admin, Status: 1}
	member := &models.User{UserEntity: &models.UserEntity{UserID: 2, TenantID: 1, UserName: "member", Email: "member", Role: defs.UserRole_Normal, LanguageGroupID: "a", Status: 1}}
	db.Create(&models.User{UserEntity: admin})
	db.Create(member)
	db.Create(&models.LanguageGroup{ID: "a", TenantID: 1, Name: "A"})
	db.Create(&models.LanguageGroup{ID: "b", TenantID: 1, Name: "B"})
	db.Create(&models.UserGroup{GroupID: "old", GroupName: "old", TenantID: 1, LanguageGroupID: "a", Users: []*models.User{member}})
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(defs.UserInfoKey, admin); c.Next() })
	OrganizationRoutes(router.Group("/org"), a)
	call := func(confirm bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]interface{}{"id": "b", "user_id": 2, "confirm": confirm})
		r := httptest.NewRequest("POST", "/org/users/move", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := call(false); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	db.First(member, 2)
	if member.LanguageGroupID != "a" {
		t.Fatal("preview changed membership")
	}
	if w := call(true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	db.First(member, 2)
	if member.LanguageGroupID != "b" || member.SessionVersion != 1 {
		t.Fatalf("move failed: %+v", member.UserEntity)
	}
	var n int64
	db.Table("user_group_memberships").Count(&n)
	if n != 0 {
		t.Fatal("old group membership retained")
	}
}
func TestSuspendKeepsProxyConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:org-suspend?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Client{}, &models.ProxyConfig{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.User{UserEntity: &models.UserEntity{UserID: 1, UserName: "one", Email: "one", LanguageGroupID: "a"}})
	original := []byte(`{"type":"tcp","name":"keep","remotePort":6000}`)
	db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "child", UserID: 1, ServerID: "s", ConfigContent: []byte(`{"serverAddr":"example","proxies":[{"name":"keep"}]}`)}})
	db.Create(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: "child", ServerID: "s", UserID: 1, Name: "keep", Content: original}})
	err = db.Transaction(func(tx *gorm.DB) error {
		return suspendOrganizationTunnels(tx, tx.Model(&models.User{}).Select("user_id").Where("language_group_id = ?", "a"), []string{"s"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var p models.ProxyConfig
	db.First(&p)
	if !p.Stopped || !bytes.Equal(p.Content, original) {
		t.Fatal("proxy config not preserved")
	}
	var c models.Client
	db.First(&c)
	if !c.Stopped {
		t.Fatal("offline reconnect will not receive stop state")
	}
}

func TestGroupAdminCannotManagePeerOrForeignAccounts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:org-manager-matrix?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.OrganizationAudit{}); err != nil {
		t.Fatal(err)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	manager := &models.UserEntity{UserID: 1, UserName: "manager", Email: "m", TenantID: 1, LanguageGroupID: "a", Role: defs.UserRole_GroupAdmin, Status: 1}
	rows := []*models.UserEntity{manager, {UserID: 2, UserName: "peer", Email: "p", TenantID: 1, LanguageGroupID: "a", Role: defs.UserRole_GroupAdmin, Status: 1}, {UserID: 3, UserName: "own", Email: "o", TenantID: 1, LanguageGroupID: "a", Role: defs.UserRole_Normal, Status: 1}, {UserID: 4, UserName: "foreign", Email: "f", TenantID: 1, LanguageGroupID: "b", Role: defs.UserRole_Normal, Status: 1}}
	for _, u := range rows {
		if err = db.Create(&models.User{UserEntity: u}).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Create(&models.LanguageGroup{ID: "a", Name: "A", TenantID: 1})
	db.Create(&models.LanguageGroup{ID: "b", Name: "B", TenantID: 1})
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(defs.UserInfoKey, manager); c.Next() })
	router.POST("/update", UpdateUser(a))
	OrganizationRoutes(router.Group("/org"), a)
	call := func(path, body string) int {
		r := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w.Code
	}
	for _, body := range []string{`{"user_id":2,"status":2}`, `{"user_id":4,"status":2}`, `{"user_id":3,"role":"admin"}`} {
		if call("/update", body) != 403 {
			t.Fatalf("forbidden mutation accepted: %s", body)
		}
	}
	if call("/update", `{"user_id":3,"status":2}`) != 200 {
		t.Fatal("own ordinary user management denied")
	}
	if call("/org/groups/save", `{"id":"b","shared":true}`) != 403 {
		t.Fatal("foreign group settings accepted")
	}
	if call("/org/groups/save", `{"id":"a","shared":true}`) != 200 {
		t.Fatal("own group sharing denied")
	}
	if call("/org/users/create-admin", `{"id":"a","username":"extra","email":"x@example.com"}`) != 403 {
		t.Fatal("group admin created peer admin")
	}
}
