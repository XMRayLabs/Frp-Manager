package shared

import (
	"crypto/sha256"
	"fmt"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
)

func TestAutoJoinPersistsBothRoles(t *testing.T) {
	for _, role := range []defs.AppRole{defs.AppRole_Client, defs.AppRole_Server} {
		t.Run(string(role), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("APPDATA", dir)
			t.Setenv("XDG_CONFIG_HOME", dir)
			count, created := 0, false
			panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count++
				if r.Header.Get(defs.AuthorizationKey) != "enroll-token" {
					t.Error("missing enrollment token")
				}
				data, _ := io.ReadAll(r.Body)
				status := &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS}
				var response proto.Message
				switch r.URL.Path {
				case "/api/v1/client/init":
					req := &pb.InitClientRequest{}
					_ = proto.Unmarshal(data, req)
					if req.GetEphemeral() {
						t.Error("node must be persistent")
					}
					created = true
					response = &pb.InitClientResponse{Status: status, ClientId: lo.ToPtr("owner.c.node")}
				case "/api/v1/server/init":
					created = true
					response = &pb.InitServerResponse{Status: status, ServerId: lo.ToPtr("owner.s.node")}
				case "/api/v1/client/get":
					if !created {
						w.WriteHeader(404)
						return
					}
					response = &pb.GetClientResponse{Status: status, Client: &pb.Client{Id: lo.ToPtr("owner.c.node"), Secret: lo.ToPtr("node-secret")}}
				case "/api/v1/server/get":
					if !created {
						w.WriteHeader(404)
						return
					}
					response = &pb.GetServerResponse{Status: status, Server: &pb.Server{Id: lo.ToPtr("owner.s.node"), Secret: lo.ToPtr("node-secret")}}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
					return
				}
				encoded, _ := proto.Marshal(response)
				w.Header().Set("Content-Type", "application/x-protobuf")
				_, _ = w.Write(encoded)
			}))
			defer panel.Close()
			cfg := conf.Config{}
			cfg.Client.APIUrl = panel.URL
			args := CommonArgs{JoinToken: lo.ToPtr("enroll-token"), Ephemeral: lo.ToPtr(false)}
			joined, err := autoJoinNode(cfg, args, role)
			if err != nil {
				t.Fatal(err)
			}
			if joined.Client.Secret != "node-secret" {
				t.Fatal("missing saved credentials")
			}
			firstCount := count
			// An expired/missing join token must not prevent the same node restarting.
			args.JoinToken = nil
			again, err := autoJoinNode(cfg, args, role)
			if err != nil {
				t.Fatal(err)
			}
			if again.Client.ID != joined.Client.ID || count != firstCount {
				t.Fatal("restart registered a new node")
			}
		})
	}
}

func TestWriteIdentityReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node.json")
	if err := writeNodeIdentity(path, nodeIdentity{Name: "pending"}); err != nil {
		t.Fatal(err)
	}
	if err := writeNodeIdentity(path, nodeIdentity{Name: "pending", ID: "registered", Secret: "secret"}); err != nil {
		t.Fatal(err)
	}
}

func TestJoinRejectsMissingToken(t *testing.T) {
	if checkPullParams(CommonArgs{}) == nil {
		t.Fatal("missing token accepted")
	}
}

func TestAutoJoinFailureDoesNotSaveCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer panel.Close()
	cfg := conf.Config{}
	cfg.Client.APIUrl = panel.URL
	for _, role := range []defs.AppRole{defs.AppRole_Client, defs.AppRole_Server} {
		if _, err := autoJoinNode(cfg, CommonArgs{JoinToken: lo.ToPtr("invalid")}, role); err == nil {
			t.Fatal("failed enrollment accepted")
		}
		if _, err := autoJoinNode(cfg, CommonArgs{}, role); err == nil {
			t.Fatal("failed enrollment persisted usable credentials")
		}
	}
}

func TestAutoJoinEphemeralDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoded, _ := proto.Marshal(&pb.GetClientResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS}, Client: &pb.Client{Id: lo.ToPtr("temporary"), Secret: lo.ToPtr("secret")}})
		_, _ = w.Write(encoded)
	}))
	defer panel.Close()
	cfg := conf.Config{}
	cfg.Client.APIUrl = panel.URL
	if _, err := autoJoinNode(cfg, CommonArgs{JoinToken: lo.ToPtr("token"), Ephemeral: lo.ToPtr(true)}, defs.AppRole_Client); err != nil {
		t.Fatal(err)
	}
	if _, err := autoJoinNode(cfg, CommonArgs{}, defs.AppRole_Client); err == nil {
		t.Fatal("temporary node persisted")
	}
}

func TestExplicitEnrollmentRecoversRevokedIdentity(t *testing.T) {
	for _, role := range []defs.AppRole{defs.AppRole_Client, defs.AppRole_Server} {
		t.Run(string(role), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("APPDATA", dir)
			t.Setenv("XDG_CONFIG_HOME", dir)
			requests, created := 0, false
			mode := "offline"
			panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				var response proto.Message
				ok := &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS}
				if req.URL.Path == "/api/v1/auth/cert" {
					if mode == "offline" {
						w.WriteHeader(503)
						return
					}
					if mode == "valid" {
						response = &pb.GetClientCertResponse{Status: ok, Cert: []byte("certificate")}
					} else {
						w.WriteHeader(500)
						response = &pb.GetClientCertResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: rpc.ErrDeviceCredentials.Error()}}
					}
				} else {
					if req.Header.Get(defs.AuthorizationKey) != "enroll-token" {
						t.Error("missing enrollment token")
					}
					switch req.URL.Path {
					case "/api/v1/client/init":
						created = true
						response = &pb.InitClientResponse{Status: ok, ClientId: lo.ToPtr("owner.c.fresh")}
					case "/api/v1/server/init":
						created = true
						response = &pb.InitServerResponse{Status: ok, ServerId: lo.ToPtr("owner.s.fresh")}
					case "/api/v1/client/get":
						if !created {
							w.WriteHeader(404)
							return
						}
						response = &pb.GetClientResponse{Status: ok, Client: &pb.Client{Id: lo.ToPtr("owner.c.fresh"), Secret: lo.ToPtr("fresh-secret")}}
					case "/api/v1/server/get":
						if !created {
							w.WriteHeader(404)
							return
						}
						response = &pb.GetServerResponse{Status: ok, Server: &pb.Server{Id: lo.ToPtr("owner.s.fresh"), Secret: lo.ToPtr("fresh-secret")}}
					default:
						t.Errorf("unexpected endpoint: %s", req.URL.Path)
						w.WriteHeader(404)
						return
					}
				}
				data, _ := proto.Marshal(response)
				w.Write(data)
			}))
			defer panel.Close()
			cfg := conf.Config{}
			cfg.Client.APIUrl = panel.URL
			cfg.Client.JoinToken = "enroll-token"
			cfg.Client.EnrollmentAttempt = "first"
			key := sha256.Sum256([]byte(fmt.Sprintf("%s|%v|", panel.URL, role)))
			path := filepath.Join(dir, "frp-manager", fmt.Sprintf("node-%x.json", key[:12]))
			identity := nodeIdentity{API: panel.URL, Role: role, Name: "original", ID: "old-name", Secret: "old-secret", Attempt: "first"}
			if err := writeNodeIdentity(path, identity); err != nil {
				t.Fatal(err)
			}
			got, err := autoJoinNode(cfg, CommonArgs{}, role)
			if err != nil || got.Client.Secret != "old-secret" || requests != 0 {
				t.Fatal("restart must reuse identity without enrollment", err)
			}
			cfg.Client.EnrollmentAttempt = "second"
			before, _ := os.ReadFile(path)
			if _, err = autoJoinNode(cfg, CommonArgs{}, role); err == nil {
				t.Fatal("outage must fail safely")
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) || created {
				t.Fatal("outage changed identity")
			}
			mode = "valid"
			got, err = autoJoinNode(cfg, CommonArgs{}, role)
			if err != nil || got.Client.Secret != "old-secret" || created {
				t.Fatal("valid identity should be retained", err)
			}
			mode = "revoked"
			n := requests
			if _, err = autoJoinNode(cfg, CommonArgs{}, role); err != nil || requests != n {
				t.Fatal("ordinary restart resurrected revoked identity", err)
			}
			cfg.Client.EnrollmentAttempt = "third"
			got, err = autoJoinNode(cfg, CommonArgs{}, role)
			if err != nil || got.Client.Secret != "fresh-secret" || !created {
				t.Fatal("explicit attempt did not recover revoked identity", err)
			}
			n = requests
			if _, err = autoJoinNode(cfg, CommonArgs{}, role); err != nil || requests != n {
				t.Fatal("successful enrollment was repeated", err)
			}
		})
	}
}
