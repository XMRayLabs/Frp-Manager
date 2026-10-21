package security_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"encoding/json"
	masterapi "github.com/Sakurame1/frp-manager/biz/master"
	masterauth "github.com/Sakurame1/frp-manager/biz/master/auth"
	clientapi "github.com/Sakurame1/frp-manager/biz/master/client"
	"github.com/Sakurame1/frp-manager/biz/master/permission"
	serverapi "github.com/Sakurame1/frp-manager/biz/master/server"
	userapi "github.com/Sakurame1/frp-manager/biz/master/user"
	workerapi "github.com/Sakurame1/frp-manager/biz/master/worker"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/middleware"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"

	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/services/workerd"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/samber/lo"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
)

// Security regressions use temporary SQLite and fake device streams only.
func fixture(t *testing.T) (app.Application, *gorm.DB, *models.UserEntity) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: glog.Default.LogMode(glog.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.Client{}, &models.Server{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.EnrollmentToken{}, &models.OrganizationAudit{}, &models.Worker{}); err != nil {
		t.Fatal(err)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	cfg := conf.Config{}
	cfg.App.GlobalSecret = "synthetic-audit-secret"
	cfg.App.CookieAge = 3600
	cfg.App.CookieName = "audit-session"
	cfg.App.CookieHTTPOnly = true
	cfg.App.CookieSecure = true
	cfg.App.CookiePath = "/"
	cfg.Master.CacheSize = 1
	a.SetConfig(cfg)
	a.SetClientsManager(rpc.NewClientsManager())
	a.SetClientRecvMap(&sync.Map{})
	owner := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "audituser", Email: "audit@example.test", Role: defs.UserRole_Normal, Status: models.STATUS_NORMAL, LanguageGroupID: "group-a", Token: "synthetic-old-frp-token"}
	if err = db.Create(&models.User{UserEntity: owner}).Error; err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"group-a", "group-b"} {
		if err = db.Create(&models.LanguageGroup{ID: g, TenantID: 1, Name: g}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return a, db, owner
}
func addClient(t *testing.T, db *gorm.DB, id string, owner int) {
	t.Helper()
	if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: id, UserID: owner, TenantID: 1, ConnectSecret: "synthetic-device-secret", ConfigContent: []byte(`{}`)}}).Error; err != nil {
		t.Fatal(err)
	}
}
func authRouter(a app.Application) *gin.Engine {
	r := gin.New()
	r.Use(middleware.SecurityHeaders(), middleware.JWTAuth(a), middleware.AuthCtx(a), middleware.RBAC(a))
	return r
}
func request(r *gin.Engine, path, body, token, origin, ctype string, cookie bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "https://panel.example.test"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	if cookie {
		req.AddCookie(&http.Cookie{Name: "audit-session", Value: token})
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Sec-Fetch-Site", "same-site")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}
func TestEnrollmentRequiresDeviceProof(t *testing.T) {
	a, db, u := fixture(t)
	proof := "test-device-secret-32-bytes-minimum-length"
	id := "audituser.c.existing"
	addClient(t, db, id, u.UserID)
	db.Model(&models.Client{}).Where("client_id = ?", id).Update("connect_secret", proof)
	token, err := models.GetEnrollmentToken(db, u.UserID, "client")
	if err != nil {
		t.Fatal(err)
	}
	r := authRouter(a)
	r.POST("/api/v1/client/get", app.Wrapper(a, clientapi.GetClientHandler))
	for _, attempt := range []string{"", "test-wrong-device-secret-32-bytes-minimum-length"} {
		body := fmt.Sprintf(`{"clientId":%q,"deviceSecret":%q}`, id, attempt)
		rec := request(r, "/api/v1/client/get", body, token, "", "application/json", false)
		if bytes.Contains(rec.Body.Bytes(), []byte(proof)) {
			t.Fatal("enrollment token disclosed existing credentials")
		}
	}
	rec := request(r, "/api/v1/client/get", fmt.Sprintf(`{"clientId":%q,"deviceSecret":%q}`, id, proof), token, "", "application/json", false)
	if !bytes.Contains(rec.Body.Bytes(), []byte(proof)) {
		t.Fatal("device proof denied", rec.Body.String())
	}
	// The same proof recovers a renamed device, without keeping old-name aliases.
	db.Model(&models.Client{}).Where("client_id = ?", id).Update("client_id", "audituser.c.renamed")
	rec = request(r, "/api/v1/client/get", fmt.Sprintf(`{"clientId":%q,"deviceSecret":%q}`, id, proof), token, "", "application/json", false)
	if !bytes.Contains(rec.Body.Bytes(), []byte("audituser.c.renamed")) {
		t.Fatal("rename recovery failed", rec.Body.String())
	}
	db.Model(&models.User{}).Where("user_id = ?", 1).Update("role", defs.UserRole_Admin)
	sid := "audituser.s.existing"
	db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: sid, TenantID: 1, UserID: 1, ConnectSecret: proof}})
	stoken, err := models.GetEnrollmentToken(db, 1, "server")
	if err != nil {
		t.Fatal(err)
	}
	r.POST("/api/v1/server/get", app.Wrapper(a, serverapi.GetServerHandler))
	rec = request(r, "/api/v1/server/get", `{"serverId":"`+sid+`"}`, stoken, "", "application/json", false)
	if bytes.Contains(rec.Body.Bytes(), []byte(proof)) {
		t.Fatal("server secret disclosed")
	}
	rec = request(r, "/api/v1/server/get", fmt.Sprintf(`{"serverId":%q,"deviceSecret":%q}`, sid, proof), stoken, "", "application/json", false)
	if !bytes.Contains(rec.Body.Bytes(), []byte(proof)) {
		t.Fatal("server proof denied", rec.Body.String())
	}
}
func TestFRPEnforcesServerGrantPortsAndRevocation(t *testing.T) {
	a, db, u := fixture(t)
	ctx := app.NewContext(context.Background(), a)
	sid := "admin.s.unassigned"
	db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: sid, UserID: 99, TenantID: 1, ConnectSecret: "server-proof", ConfigContent: []byte(`{"vhostHTTPPort":443}`)}})
	req := &pb.FRPAuthRequest{User: u.UserName, Token: u.Token, Operation: "Login", Base: &pb.ServerBase{ServerId: sid, ServerSecret: "wrong"}}
	check := func(want bool) {
		t.Helper()
		res, err := serverapi.FRPAuth(ctx, req)
		if want && (err != nil || !res.GetOk()) || !want && err == nil && res.GetOk() {
			t.Fatalf("operation=%s type=%s port=%d got=%v err=%v want=%v", req.Operation, req.ProxyType, req.RemotePort, res, err, want)
		}
	}
	check(false)
	req.Base.ServerSecret = "server-proof"
	check(false)
	grant := &models.LanguageGroupServer{LanguageGroupID: u.LanguageGroupID, ServerID: sid, AllowedPorts: "62000-63999"}
	if err := db.Create(grant).Error; err != nil {
		t.Fatal(err)
	}
	check(true)
	req.Operation = "NewProxy"
	req.ProxyType = "tcp"
	req.RemotePort = 62000
	check(true)
	req.RemotePort = 63999
	check(true)
	req.RemotePort = 64000
	check(false)
	req.RemotePort = 0
	check(false)
	req.ProxyType = "http"
	check(false)
	req.Operation = "unknown"
	check(false)
	req.Operation = "Ping"
	check(true)
	db.Model(&models.User{}).Where("user_id = ?", u.UserID).Update("token", "rotated-frp-token")
	check(false)
	req.Token = "rotated-frp-token"
	check(true)
	db.Model(&models.User{}).Where("user_id = ?", u.UserID).Update("status", models.STATUS_BANED)
	check(false)
	db.Model(&models.User{}).Where("user_id = ?", u.UserID).Update("status", models.STATUS_NORMAL)
	db.Where("language_group_id = ? AND server_id = ?", u.LanguageGroupID, sid).Delete(&models.LanguageGroupServer{})
	check(false)
	req.Base = nil
	check(false)
	ops := conf.FRPsAuthOption(a.GetConfig()).Ops
	for _, want := range []string{"Login", "NewProxy", "Ping", "NewWorkConn"} {
		found := false
		for _, op := range ops {
			found = found || op == want
		}
		if !found {
			t.Fatal("missing authentication hook", want)
		}
	}
}
func TestBrowserOriginAndContentTypeProtection(t *testing.T) {
	a, db, u := fixture(t)
	id := "audituser.c.private"
	addClient(t, db, id, u.UserID)
	token := conf.GetJWTWithAllPermission(a.GetConfig(), u.UserID)
	r := authRouter(a)
	permission.OrganizationRoutes(r.Group("/api/v1/organization"), a)
	body := `{"client_id":"` + id + `","private":true}`
	for _, typ := range []string{"application/json", "text/plain"} {
		rec := request(r, "/api/v1/organization/clients/privacy", body, token, "https://untrusted.example.test", typ, true)
		if rec.Code != 403 {
			t.Fatal("cross-origin request accepted", rec.Code, rec.Body.String())
		}
	}
	var c models.Client
	db.Where("client_id = ?", id).First(&c)
	if c.Private {
		t.Fatal("rejected request mutated device")
	}
	rec := request(r, "/api/v1/organization/clients/privacy", body, token, "https://panel.example.test", "text/plain", true)
	if rec.Code != 415 {
		t.Fatal("simple request MIME accepted", rec.Code)
	}
	rec = request(r, "/api/v1/organization/clients/privacy", body, token, "https://panel.example.test", "application/json", true)
	if rec.Code != 200 {
		t.Fatal("same-origin JSON denied", rec.Code, rec.Body.String())
	}
	rec = request(r, "/api/v1/organization/clients/privacy", body, token, "", "application/json", false)
	if rec.Code != 200 {
		t.Fatal("native bearer JSON denied", rec.Code)
	}
}
func TestWorkerPathsStayInsideRoot(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "runtime")
	for _, entry := range []string{"../../../../outside-marker.txt", `..\..\outside-marker.txt`, "/absolute", "C:/absolute", "a/../entry.js"} {
		worker := &pb.Worker{WorkerId: lo.ToPtr("safe-worker"), CodeEntry: &entry, Code: lo.ToPtr("marker")}
		if err := workerd.WriteWorkerCodeToFile(context.Background(), worker, cwd); err == nil {
			t.Fatal("escaped path accepted", entry)
		}
	}
	entry := "nested/index.js"
	worker := &pb.Worker{WorkerId: lo.ToPtr("safe-worker"), CodeEntry: &entry, Code: lo.ToPtr("marker")}
	if err := workerd.WriteWorkerCodeToFile(context.Background(), worker, cwd); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(workerd.CodeFilePath(context.Background(), worker, cwd)); err != nil || string(b) != "marker" {
		t.Fatal(err)
	}
	worker.WorkerId = lo.ToPtr("../../outside")
	if err := workerd.WriteWorkerCodeToFile(context.Background(), worker, cwd); err == nil {
		t.Fatal("escaped worker ID accepted")
	}
	worker.WorkerId = lo.ToPtr("safe-worker")
	worker.ConfigTemplate = lo.ToPtr("{{invalid template")
	if err := workerd.GenWorkerConfig(worker, cwd); err == nil {
		t.Fatal("invalid template accepted")
	}
}
func TestWorkerSymlinkCannotEscapeRoot(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "runtime")
	outside := filepath.Join(root, "outside")
	os.MkdirAll(filepath.Join(cwd, "workers"), 0700)
	os.MkdirAll(outside, 0700)
	if err := os.Symlink(outside, filepath.Join(cwd, "workers", "symlink-worker")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	worker := &pb.Worker{WorkerId: lo.ToPtr("symlink-worker"), CodeEntry: lo.ToPtr("index.js"), Code: lo.ToPtr("marker")}
	if err := workerd.WriteWorkerCodeToFile(context.Background(), worker, cwd); err == nil {
		t.Fatal("symlink escape accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "src", "index.js")); !os.IsNotExist(err) {
		t.Fatal("outside file created", err)
	}
}

type captureStream struct {
	a      app.Application
	events chan pb.Event
}

func (f *captureStream) Send(m *pb.ServerMessage) error {
	f.events <- m.Event
	var data []byte
	if m.Event == pb.Event_EVENT_REMOVE_WORKER {
		data, _ = proto.Marshal(&pb.RemoveWorkerResponse{})
	} else {
		data, _ = proto.Marshal(&pb.CreateWorkerResponse{})
	}
	if ch, ok := f.a.GetClientRecvMap().Load(m.SessionId); ok {
		ch.(chan *pb.ClientMessage) <- &pb.ClientMessage{Event: m.Event, Data: data, SessionId: m.SessionId}
	}
	return nil
}
func (*captureStream) Recv() (*pb.ClientMessage, error) { return nil, io.EOF }
func (*captureStream) SetHeader(metadata.MD) error      { return nil }
func (*captureStream) SendHeader(metadata.MD) error     { return nil }
func (*captureStream) SetTrailer(metadata.MD)           {}
func (*captureStream) Context() context.Context         { return context.Background() }
func (*captureStream) SendMsg(any) error                { return nil }
func (*captureStream) RecvMsg(any) error                { return io.EOF }
func TestWorkerCannotUseRevokedDeviceAssociation(t *testing.T) {
	a, db, member := fixture(t)
	admin := &models.UserEntity{UserID: 2, TenantID: 1, UserName: "groupadmin", Email: "groupadmin@example.test", Role: defs.UserRole_GroupAdmin, Status: models.STATUS_NORMAL, LanguageGroupID: "group-a"}
	if err := db.Create(&models.User{UserEntity: admin}).Error; err != nil {
		t.Fatal(err)
	}
	id := "audituser.c.moved"
	addClient(t, db, id, member.UserID)
	var c models.Client
	if err := db.Where("client_id = ?", id).First(&c).Error; err != nil {
		t.Fatal(err)
	}
	worker := &models.Worker{WorkerEntity: &models.WorkerEntity{ID: "existing-worker", Name: "audit", UserId: 2, TenantId: 1, CodeEntry: "index.js", Code: "// harmless"}, Clients: []models.Client{c}}
	if err := db.Create(worker).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.User{}).Where("user_id = ?", member.UserID).Update("language_group_id", "group-b").Error; err != nil {
		t.Fatal(err)
	}
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, admin), a)
	if dao.CanManageClient(ctx, admin, id) == nil {
		t.Fatal("old group admin still authorized by fixture")
	}
	stream := &captureStream{a: a, events: make(chan pb.Event, 4)}
	a.GetClientsManager().Set(id, defs.CliTypeClient, stream, nil)
	if _, err := workerapi.RedeployWorker(ctx, &pb.RedeployWorkerRequest{WorkerId: lo.ToPtr(worker.ID)}); err == nil {
		t.Fatal("old administrator retained deployment rights")
	}
	select {
	case event := <-stream.events:
		t.Fatal("unauthorized event dispatched", event)
	default:
	}
	if rows, err := dao.NewQuery(ctx).AdminListWorkersByClientID(id); err != nil || len(rows) != 0 {
		t.Fatal("device pull resurrects unauthorized worker", len(rows), err)
	}
	db.Model(&models.User{}).Where("user_id = ?", member.UserID).Update("language_group_id", "group-a")
	if _, err := workerapi.RedeployWorker(ctx, &pb.RedeployWorkerRequest{WorkerId: lo.ToPtr(worker.ID)}); err != nil {
		t.Fatal("authorized group administrator denied", err)
	}
	for _, want := range []pb.Event{pb.Event_EVENT_REMOVE_WORKER, pb.Event_EVENT_CREATE_WORKER} {
		select {
		case got := <-stream.events:
			if got != want {
				t.Fatal(got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("missing authorized fake device event")
		}
	}

}
func TestSQLInputsRemainBound(t *testing.T) {
	a, db, u := fixture(t)
	addClient(t, db, "audituser.c.mine", u.UserID)
	addClient(t, db, "other.c.secret", 999)
	q := dao.NewQuery(app.NewContext(context.Background(), a))
	for _, payload := range []string{"' OR 1=1 --", "' UNION SELECT 1 --", "'; DROP TABLE clients; --"} {
		if _, err := q.GetUserByUserName(payload); err == nil {
			t.Fatalf("username injection matched %q", payload)
		}
		if _, err := q.GetClientByClientID(u, payload); err == nil {
			t.Fatalf("ID injection matched %q", payload)
		}
		rows, err := q.ListClientsWithKeyword(u, 1, 100, payload)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 0 {
			t.Fatalf("keyword injection matched %q", payload)
		}
	}
	var count int64
	if err := db.Model(&models.Client{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("database altered", count, err)
	}
	t.Log(fmt.Sprintf("NO REPRODUCTION: SQL-injection strings remain literal in username/ID/keyword queries; %d client rows intact; isolation unchanged.", count))
}

func TestOtherUsersCannotInstallWorkerRuntime(t *testing.T) {
	a, db, u := fixture(t)
	id := "other.c.private"
	addClient(t, db, id, 999)
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, u), a)
	if _, err := workerapi.InstallWorkerd(ctx, &pb.InstallWorkerdRequest{ClientId: lo.ToPtr(id), DownloadUrl: lo.ToPtr("https://example.invalid/never-requested")}); err == nil {
		t.Fatal("foreign device execution path accepted")
	}
	t.Log("NO REPRODUCTION: ordinary user is denied runtime installation on another user's device before download or RPC dispatch.")
}

func TestAnonymousCannotReachDeviceExecutionEndpoints(t *testing.T) {
	a, _, _ := fixture(t)
	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	masterapi.ConfigureRouter(a, r)
	for _, path := range []string{"/api/v1/client/install_workerd", "/api/v1/client/upgrade", "/api/v1/worker/create", "/api/v1/worker/redeploy"} {
		rec := request(r, path, `{}`, "", "", "application/json", false)
		if rec.Code != 401 {
			t.Fatal("anonymous endpoint not denied", path, rec.Code)
		}
	}
	req := httptest.NewRequest("GET", "https://panel.example.test/api/v1/pty/foreign-device", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal("anonymous terminal not denied", rec.Code)
	}
}

func TestSecureEnrollmentThroughRealHTTPHandlers(t *testing.T) {
	for _, role := range []string{"client", "server"} {
		t.Run(role, func(t *testing.T) {
			a, db, u := fixture(t)
			if role == "server" {
				db.Model(&models.User{}).Where("user_id = ?", u.UserID).Update("role", defs.UserRole_Admin)
			}
			token, err := models.GetEnrollmentToken(db, u.UserID, role)
			if err != nil {
				t.Fatal(err)
			}
			r := authRouter(a)
			r.POST("/api/v1/client/init", app.Wrapper(a, clientapi.InitClientHandler))
			r.POST("/api/v1/client/get", app.Wrapper(a, clientapi.GetClientHandler))
			r.POST("/api/v1/server/init", app.Wrapper(a, serverapi.InitServerHandler))
			r.POST("/api/v1/server/get", app.Wrapper(a, serverapi.GetServerHandler))
			panel := httptest.NewServer(r)
			defer panel.Close()
			cfg := a.GetConfig()
			cfg.Client.APIUrl = panel.URL
			proof := "test-secure-enrollment-device-proof-32-bytes"
			id := "audituser.c.new-device"
			if role == "client" {
				result, err := rpc.InitClient(cfg, "new-device", token, nil, proof)
				if err != nil || result.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
					t.Fatal(result, err)
				}
				id = result.GetClientId()
				node, err := rpc.GetClient(cfg, id, token, proof)
				if err != nil || node.GetClient().GetSecret() != proof {
					t.Fatal(node, err)
				}
				denied, err := rpc.GetClient(cfg, id, token)
				if err == nil && denied.GetClient().GetSecret() != "" {
					t.Fatal("legacy request leaked device credentials")
				}
			} else {
				node, err := rpc.JoinServer(cfg, "new-device", token, proof)
				if err != nil || node.GetSecret() != proof {
					t.Fatal(node, err)
				}
				id = node.GetId()
				again, err := rpc.JoinServer(cfg, "new-device", token, proof)
				if err != nil || again.GetId() != id {
					t.Fatal("retry must reuse existing server", again, err)
				}
			}
			// A fixed enrollment token must not permit legacy initialization without proof.
			body := `{"clientId":"legacy-device"}`
			if role == "server" {
				body = `{"serverId":"legacy-device","serverIp":"127.0.0.1"}`
			}
			denied := request(r, "/api/v1/"+role+"/init", body, token, "", "application/json", false)
			var count int64
			if role == "client" {
				db.Model(&models.Client{}).Where("client_id = ?", "audituser.c.legacy-device").Count(&count)
			} else {
				db.Model(&models.Server{}).Where("server_id = ?", "audituser.s.legacy-device").Count(&count)
			}
			if count != 0 {
				t.Fatal("legacy enrollment created device without proof", denied.Body.String())
			}
		})
	}
}

func TestDefaultServerPluginCannotBypassAuthorization(t *testing.T) {
	a, db, u := fixture(t)
	dao.NewMutation(app.NewContext(context.Background(), a)).InitDefaultServer("127.0.0.1")
	r := gin.New()
	r.POST("/auth", masterauth.MakeGinHandlerFunc(a, masterauth.HandleLogin))
	call := func(op, body string) bool {
		rec := request(r, "/auth", `{"op":"`+op+`","content":`+body+`}`, "", "", "application/json", false)
		var response struct {
			Reject   bool `json:"reject"`
			Unchange bool `json:"unchange"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &response) != nil {
			t.Fatal(rec.Code, rec.Body.String())
		}
		return !response.Reject && response.Unchange
	}
	login := fmt.Sprintf(`{"user":%q,"metas":{"token":%q}}`, u.UserName, u.Token)
	if call("Login", login) {
		t.Fatal("unassigned default server accepted login")
	}
	grant := models.LanguageGroupServer{LanguageGroupID: models.GroupID(u), ServerID: defs.DefaultServerID, AllowedPorts: "62000-63999"}
	if err := db.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	if !call("Login", login) {
		t.Fatal("assigned server rejected login")
	}
	proxy := func(port int) string {
		return fmt.Sprintf(`{"user":%s,"proxy_type":"tcp","remote_port":%d}`, login, port)
	}
	if call("NewProxy", proxy(61999)) {
		t.Fatal("default server accepted forbidden port")
	}
	if !call("NewProxy", proxy(62000)) {
		t.Fatal("default server rejected allowed port")
	}
	if call("NewProxy", proxy(65536)) || call("Unrecognized", login) {
		t.Fatal("invalid plugin request accepted")
	}
	db.Model(&models.User{}).Where("user_id = ?", u.UserID).Update("token", "rotated-token")
	if call("Login", login) {
		t.Fatal("old token still accepted by default plugin")
	}
}

func TestRestrictedTokenCannotMintBroaderOrLongerTokens(t *testing.T) {
	a, _, u := fixture(t)
	scope := []defs.APIPermission{{Method: "POST", Path: "^/api/v1/user/sign-token$"}, {Method: "POST", Path: "^/api/v1/client/list$"}}
	token, err := utils.GetJwtTokenFromMap(conf.JWTSecret(a.GetConfig()), time.Now().Unix(), 120, map[string]interface{}{defs.UserIDKey: u.UserID, defs.TokenPayloadKey_Permissions: scope, "session_version": 0})
	if err != nil {
		t.Fatal(err)
	}
	r := authRouter(a)
	r.POST("/api/v1/user/sign-token", app.Wrapper(a, userapi.SignTokenHandler))
	for _, body := range []string{
		`{"expiresIn":60,"permissions":[{"method":"*","path":"*"}]}`,
		`{"expiresIn":3600,"permissions":[{"method":"POST","path":"^/api/v1/client/list$"}]}`,
		`{"expiresIn":60,"permissions":[{"method":"POST","path":"^/api/v1/client/.*$"}]}`,
	} {
		rec := request(r, "/api/v1/user/sign-token", body, token, "", "application/json", false)
		if bytes.Contains(rec.Body.Bytes(), []byte(`"token":`)) {
			t.Fatal("restricted token minted broader credentials", rec.Body.String())
		}
	}
	rec := request(r, "/api/v1/user/sign-token", `{"expiresIn":60,"permissions":[{"method":"POST","path":"^/api/v1/client/list$"}]}`, token, "", "application/json", false)
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatal("restricted token was automatically renewed")
	}

	if !bytes.Contains(rec.Body.Bytes(), []byte(`"token":`)) {
		t.Fatal("narrower delegation denied", rec.Body.String())
	}
}

func TestAPITokenKeepsItsExplicitExpiration(t *testing.T) {
	a, _, u := fixture(t)
	token, err := conf.GetJWTWithPayload(a.GetConfig(), u.UserID, map[string]interface{}{defs.TokenPayloadKey_Permissions: conf.AllPermission(), "session_version": 0})
	if err != nil {
		t.Fatal(err)
	}
	r := authRouter(a)
	r.POST("/api/v1/user/sign-token", app.Wrapper(a, userapi.SignTokenHandler))
	issued := request(r, "/api/v1/user/sign-token", `{"expiresIn":120,"permissions":[{"method":"*","path":"*"}]}`, token, "", "application/json", false)
	var output struct {
		Body struct {
			Token string `json:"token"`
		} `json:"body"`
	}
	if err := json.Unmarshal(issued.Body.Bytes(), &output); err != nil || output.Body.Token == "" {
		t.Fatal("interactive API issuance failed", issued.Body.String(), err)
	}
	claims, err := utils.ParseToken(conf.JWTSecret(a.GetConfig()), output.Body.Token)
	if err != nil || claims["api_token"] != true {
		t.Fatal("API credential not marked", err)
	}
	for _, cookie := range []bool{false, true} {
		rec := request(r, "/api/v1/user/sign-token", `{"expiresIn":3600,"permissions":[{"method":"*","path":"*"}]}`, output.Body.Token, "", "application/json", cookie)
		if rec.Header().Get("Set-Cookie") != "" || bytes.Contains(rec.Body.Bytes(), []byte(`"token":`)) {
			t.Fatal("API lifetime extended", rec.Body.String())
		}
	}
}
