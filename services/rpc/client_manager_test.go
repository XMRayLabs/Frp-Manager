package rpc

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"google.golang.org/grpc/metadata"
)

type fakeServerSendStream struct {
	ctx context.Context
}

func (f *fakeServerSendStream) Send(*pb.ServerMessage) error     { return nil }
func (f *fakeServerSendStream) Recv() (*pb.ClientMessage, error) { return nil, io.EOF }
func (f *fakeServerSendStream) SetHeader(metadata.MD) error      { return nil }
func (f *fakeServerSendStream) SendHeader(metadata.MD) error     { return nil }
func (f *fakeServerSendStream) SetTrailer(metadata.MD)           {}
func (f *fakeServerSendStream) Context() context.Context         { return f.ctx }
func (f *fakeServerSendStream) SendMsg(any) error                { return nil }
func (f *fakeServerSendStream) RecvMsg(any) error                { return io.EOF }

func TestClientsManagerStatusProbeLifecycle(t *testing.T) {
	manager := NewClientsManager()
	stream := &fakeServerSendStream{ctx: context.Background()}
	version := &pb.ClientVersion{GitVersion: "1.0.0"}
	connector := manager.Set("client-1", defs.CliTypeClient, stream, version)

	snapshot, ok := manager.GetRuntimeSnapshot("client-1")
	if !ok || snapshot.Version.GetGitVersion() != "1.0.0" || !snapshot.Healthy {
		t.Fatalf("unexpected initial snapshot: %#v, ok=%v", snapshot, ok)
	}
	if !manager.TryStartStatusProbe("client-1", time.Minute) {
		t.Fatal("first probe should start")
	}
	if manager.TryStartStatusProbe("client-1", time.Minute) {
		t.Fatal("duplicate probe should be rejected")
	}

	manager.FinishStatusProbe("client-1", connector, 12, version, true)
	snapshot, ok = manager.GetRuntimeSnapshot("client-1")
	if !ok || snapshot.Ping != 12 || snapshot.CheckedAt.IsZero() {
		t.Fatalf("unexpected completed snapshot: %#v, ok=%v", snapshot, ok)
	}
	if manager.TryStartStatusProbe("client-1", time.Minute) {
		t.Fatal("fresh snapshot should not be probed")
	}
}

func TestClientsManagerDoesNotRemoveReconnectedClient(t *testing.T) {
	manager := NewClientsManager()
	oldConnector := manager.Set("client-1", defs.CliTypeClient, &fakeServerSendStream{ctx: context.Background()}, nil)
	newConnector := manager.Set("client-1", defs.CliTypeClient, &fakeServerSendStream{ctx: context.Background()}, nil)

	manager.RemoveIfCurrent("client-1", oldConnector)
	if manager.Get("client-1") != newConnector {
		t.Fatal("disconnecting an old stream removed the replacement connection")
	}

	manager.RemoveIfCurrent("client-1", newConnector)
	if manager.Get("client-1") != nil {
		t.Fatal("current connection was not removed")
	}
}

func TestCallClientHonorsContextCancellation(t *testing.T) {
	appInstance := app.NewApp()
	appInstance.SetClientsManager(NewClientsManager())
	appInstance.SetClientRecvMap(&sync.Map{})
	appInstance.GetClientsManager().Set(
		"client-1",
		defs.CliTypeClient,
		&fakeServerSendStream{ctx: context.Background()},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := CallClient(app.NewContext(ctx, appInstance), "client-1", pb.Event_EVENT_PING, &pb.CommonRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}

	pending := 0
	appInstance.GetClientRecvMap().Range(func(_, _ any) bool {
		pending++
		return true
	})
	if pending != 0 {
		t.Fatalf("expected pending calls to be cleaned up, got %d", pending)
	}
}

func TestRenameRetainsStreamAndReconnectCleanup(t *testing.T) {
	manager := NewClientsManager()
	old := manager.Set("a", defs.CliTypeClient, &fakeServerSendStream{ctx: context.Background()}, &pb.ClientVersion{GitVersion: "1.0.3"})
	manager.Rename("a", "b")
	manager.Rename("b", "c")
	if manager.Get("c") != old || manager.Get("a") != old {
		t.Fatal("rename replaced stream")
	}
	if snapshot, ok := manager.GetRuntimeSnapshot("c"); !ok || snapshot.Version.GetGitVersion() != "1.0.3" {
		t.Fatal("lost snapshot")
	}
	replacement := manager.Set("a", defs.CliTypeClient, &fakeServerSendStream{ctx: context.Background()}, nil)
	manager.RemoveIfCurrent("a", old)
	if manager.Get("c") != replacement {
		t.Fatal("old stream cleanup removed new connection")
	}
	manager.RemoveIfCurrent("c", replacement)
	if manager.Get("a") != nil {
		t.Fatal("stale alias connection")
	}
}
