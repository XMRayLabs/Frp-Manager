package rpc

import (
	"sync"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"google.golang.org/grpc/peer"
)

type ClientsManager interface {
	Get(cliID string) *defs.Connector
	Set(cliID, clientType string, sender pb.Master_ServerSendServer, version *pb.ClientVersion) *defs.Connector
	Remove(cliID string)
	ClientAddr(cliID string) string
	ConnectTime(cliID string) (time.Time, bool)
}

type ClientsManagerImpl struct {
	connectionMu    sync.Mutex
	senders         *utils.SyncMap[string, *defs.Connector]
	connectTime     *utils.SyncMap[string, time.Time]
	lastSeenAt      *utils.SyncMap[string, time.Time]
	runtimeSnapshot *utils.SyncMap[string, app.ClientRuntimeSnapshot]
	statusProbes    *utils.SyncMap[string, struct{}]
}

// Get implements ClientsManager.
func (c *ClientsManagerImpl) Get(cliID string) *defs.Connector {
	cliAny, ok := c.senders.Load(cliID)
	if !ok {
		return nil
	}
	return cliAny
}

// Set implements ClientsManager.
func (c *ClientsManagerImpl) Set(cliID, clientType string, sender pb.Master_ServerSendServer, version *pb.ClientVersion) *defs.Connector {
	wireID := cliID
	if stream, ok := sender.(*IdentityStream); ok {
		wireID = stream.ID
	}
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()

	if old := c.Get(cliID); old != nil {
		old.Close()
	}
	connector := &defs.Connector{
		Done:    make(chan struct{}),
		CliID:   wireID,
		Conn:    sender,
		CliType: clientType,
	}
	c.senders.Store(cliID, connector)
	c.connectTime.Store(cliID, time.Now())
	c.lastSeenAt.Store(cliID, time.Now())
	c.runtimeSnapshot.Store(cliID, app.ClientRuntimeSnapshot{
		Ping:    -1,
		Version: version,
		Healthy: true,
	})
	c.statusProbes.Delete(cliID)
	return connector
}

func (c *ClientsManagerImpl) Remove(cliID string) {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	c.remove(cliID)
}

func (c *ClientsManagerImpl) remove(cliID string) {
	if old := c.Get(cliID); old != nil {
		old.Close()
	}
	c.senders.Delete(cliID)
	c.connectTime.Delete(cliID)
	c.lastSeenAt.Delete(cliID)
	c.runtimeSnapshot.Delete(cliID)
	c.statusProbes.Delete(cliID)
}

func (c *ClientsManagerImpl) RemoveIfCurrent(cliID string, connector *defs.Connector) {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	if c.Get(cliID) == connector {
		c.remove(cliID)
		return
	}
	c.senders.Range(func(id string, current *defs.Connector) bool {
		if current == connector {
			c.remove(id)
			return false
		}
		return true
	})
}

func (c *ClientsManagerImpl) ClientAddr(cliID string) string {
	connector := c.Get(cliID)
	if connector == nil {
		return ""
	}
	p, ok := peer.FromContext(connector.Conn.Context())
	if !ok || p == nil {
		return ""
	}
	return p.Addr.String()
}

func (c *ClientsManagerImpl) ConnectTime(cliID string) (time.Time, bool) {
	t, ok := c.connectTime.Load(cliID)
	if !ok {
		return time.Time{}, false
	}
	return t, true
}

func (c *ClientsManagerImpl) UpdateLastSeenAt(cliID string) {
	c.lastSeenAt.Store(cliID, time.Now())
}

func (c *ClientsManagerImpl) GetLastSeenAt(cliID string) (time.Time, bool) {
	t, ok := c.lastSeenAt.Load(cliID)
	if !ok {
		return time.Time{}, false
	}
	return t, true
}

func (c *ClientsManagerImpl) GetRuntimeSnapshot(cliID string) (app.ClientRuntimeSnapshot, bool) {
	return c.runtimeSnapshot.Load(cliID)
}

func (c *ClientsManagerImpl) TryStartStatusProbe(cliID string, minInterval time.Duration) bool {
	if snapshot, ok := c.runtimeSnapshot.Load(cliID); ok &&
		!snapshot.CheckedAt.IsZero() &&
		time.Since(snapshot.CheckedAt) < minInterval {
		return false
	}
	_, alreadyRunning := c.statusProbes.LoadOrStore(cliID, struct{}{})
	return !alreadyRunning
}

func (c *ClientsManagerImpl) FinishStatusProbe(
	cliID string,
	connector *defs.Connector,
	ping int32,
	version *pb.ClientVersion,
	healthy bool,
) {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	if c.Get(cliID) != connector {
		return
	}
	c.statusProbes.Delete(cliID)

	if version == nil {
		if previous, ok := c.runtimeSnapshot.Load(cliID); ok {
			version = previous.Version
		}
	}
	c.runtimeSnapshot.Store(cliID, app.ClientRuntimeSnapshot{
		Ping:      ping,
		Version:   version,
		CheckedAt: time.Now(),
		Healthy:   healthy,
	})
}

func NewClientsManager() app.ClientsManager {
	return &ClientsManagerImpl{
		senders:         &utils.SyncMap[string, *defs.Connector]{},
		connectTime:     &utils.SyncMap[string, time.Time]{},
		lastSeenAt:      &utils.SyncMap[string, time.Time]{},
		runtimeSnapshot: &utils.SyncMap[string, app.ClientRuntimeSnapshot]{},
		statusProbes:    &utils.SyncMap[string, struct{}]{},
	}
}

// Move the lookup keys without touching the stream or connector used by Recv.
func (c *ClientsManagerImpl) Rename(oldID, newID string) {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	if value, ok := c.senders.Load(oldID); ok {
		c.senders.Store(newID, value)
		c.senders.Delete(oldID)
	}
	if value, ok := c.connectTime.Load(oldID); ok {
		c.connectTime.Store(newID, value)
		c.connectTime.Delete(oldID)
	}
	if value, ok := c.lastSeenAt.Load(oldID); ok {
		c.lastSeenAt.Store(newID, value)
		c.lastSeenAt.Delete(oldID)
	}
	if value, ok := c.runtimeSnapshot.Load(oldID); ok {
		c.runtimeSnapshot.Store(newID, value)
		c.runtimeSnapshot.Delete(oldID)
	}
	c.statusProbes.Delete(oldID)
}

// IdentityStream carries the wire ID of an authenticated legacy kernel.
type IdentityStream struct {
	pb.Master_ServerSendServer
	ID string
}
