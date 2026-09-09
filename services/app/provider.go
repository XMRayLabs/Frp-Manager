package app

import (
	"sync"
	"time"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/casbin/casbin/v2"

	"github.com/fatedier/frp/client/proxy"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/metrics/mem"
	"gorm.io/gorm"
)

// biz/common/stream_log.go
type StreamLogHookMgr interface {
	AddStream(send func(msg string), closeSend func())
	SetPkgs(pkgs []string)
	Close()
	Lock()
	TryLock() bool
	Unlock()
}

// utils/sync.go
type SyncMap[K comparable, V any] interface {
	Clone() utils.SyncMap[K, V]
	Delete(key K)
	Grow(size int)
	Keys() []K
	Len() (l int)
	Load(key K) (value V, loaded bool)
	LoadAndDelete(key K) (value V, loaded bool)
	LoadOrStore(key K, value V) (actual V, loaded bool)
	Range(f func(key K, value V) (shouldContinue bool))
	Store(key K, value V)
	Values() []V
}

type GoSyncMap interface {
	Clear()
	CompareAndDelete(key any, old any) (deleted bool)
	CompareAndSwap(key any, old any, new any) (swapped bool)
	Delete(key any)
	Load(key any) (value any, ok bool)
	LoadAndDelete(key any) (value any, loaded bool)
	LoadOrStore(key any, value any) (actual any, loaded bool)
	Range(f func(key any, value any) bool)
	Store(key any, value any)
	Swap(key any, value any) (previous any, loaded bool)
}

// biz/master/shell/mgr.go
type ShellPTYMgr interface {
	SyncMap[string, pb.Master_PTYConnectServer]
	Add(sessionID string, conn pb.Master_PTYConnectServer)
	IsSessionDone(sessionID string) bool
	SetSessionDone(sessionID string)
}

// biz/master/streamlog/collect_log.go
type ClientLogManager interface {
	SyncMap[string, chan string]
	GetClientLock(clientId string) *sync.Mutex
}

// models/db.go
type DBManager interface {
	GetDB(dbType string, dbRole string) *gorm.DB
	GetDefaultDB() *gorm.DB
	SetDB(dbType string, dbRole string, db *gorm.DB)
	RemoveDB(dbType string, dbRole string)
	SetDebug(bool)
	Init()
}

type ClientsManager interface {
	Rename(oldID, newID string)
	Get(cliID string) *defs.Connector
	Set(cliID, clientType string, sender pb.Master_ServerSendServer, version *pb.ClientVersion) *defs.Connector
	Remove(cliID string)
	RemoveIfCurrent(cliID string, connector *defs.Connector)
	ClientAddr(cliID string) string
	ConnectTime(cliID string) (time.Time, bool)
	UpdateLastSeenAt(cliID string)
	GetLastSeenAt(cliID string) (time.Time, bool)
	GetRuntimeSnapshot(cliID string) (ClientRuntimeSnapshot, bool)
	TryStartStatusProbe(cliID string, minInterval time.Duration) bool
	FinishStatusProbe(cliID string, connector *defs.Connector, ping int32, version *pb.ClientVersion, healthy bool)
}

type ClientRuntimeSnapshot struct {
	Ping      int32
	Version   *pb.ClientVersion
	CheckedAt time.Time
	Healthy   bool
}

type Service interface {
	Run()
	Stop()
}

// services/client/frpc_service.go
type ClientHandler interface {
	Run()
	Stop()
	Wait()
	Running() bool
	Update([]v1.ProxyConfigurer, []v1.VisitorConfigurer)
	AddProxy(v1.ProxyConfigurer)
	AddVisitor(v1.VisitorConfigurer)
	RemoveProxy(v1.ProxyConfigurer)
	RemoveVisitor(v1.VisitorConfigurer)
	GetProxyStatus(string) (*proxy.WorkingStatus, bool)
	GetCommonCfg() *v1.ClientCommonConfig
	GetProxyCfgs() map[string]v1.ProxyConfigurer
	GetVisitorCfgs() map[string]v1.VisitorConfigurer
}

// services/rpcclient/rpc_service.go
type ClientRPCHandler interface {
	Run()
	Stop()
	GetCli() MasterClient
}

type ClientController interface {
	Add(clientID string, serverID string, clientHandler ClientHandler)
	Get(clientID string, serverID string) ClientHandler
	Delete(clientID string, serverID string)
	Set(clientID string, serverID string, clientHandler ClientHandler)
	Run(clientID string, serverID string) // 涓嶉樆濉?
	Stop(clientID string, serverID string)
	GetByClient(clientID string) *utils.SyncMap[string, ClientHandler]
	DeleteByClient(clientID string)
	RunByClient(clientID string) // 涓嶉樆濉?
	StopByClient(clientID string)
	StopAll()
	DeleteAll()
	RunAll()
	List() []string
}

type ServerController interface {
	Add(serverID string, serverHandler ServerHandler)
	Get(serverID string) ServerHandler
	Delete(serverID string)
	Set(serverID string, serverHandler ServerHandler)
	Run(serverID string) // 涓嶉樆濉?
	Stop(serverID string)
	List() []string
}

type ServerHandler interface {
	Run()
	Stop()
	IsFirstSync() bool
	GetCommonCfg() *v1.ServerConfig
	GetMem() *mem.ServerStats
	GetProxyStatsByType(v1.ProxyType) []*mem.ProxyStats
}

// rpc/master.go
type MasterClient interface {
	Call() pb.MasterClient
}

// services/rbac/perm_manager.go
type PermissionManager interface {
	AddUserToGroup(userID int, groupID string, tenantID int) (bool, error)
	CheckPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error)
	Enforcer() *casbin.Enforcer
	GrantGroupPermission(groupID string, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error)
	GrantUserPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error)
	RemoveUserFromGroup(userID int, groupID string, tenantID int) (bool, error)
	RevokeGroupPermission(groupID string, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error)
	RevokeUserPermission(userID int, objType defs.RBACObj, objID string, action defs.RBACAction, tenantID int) (bool, error)
}

// services/workerd/exec_manager.go
type WorkerExecManager interface {
	RunCmd(workerId string, cwd string, argv []string)
	ExitCmd(workerId string)
	ExitAllCmd()
	UpdateBinaryPath(path string)
}

// services/workerd/workerd.go
type WorkerController interface {
	RunWorker(c *Context)
	StopWorker(c *Context)
	// GetWorkerStatus(c *Context) defs.WorkerStatus
	GarbageCollect()
	Init(c *Context) error
}

// services/workerd/workers_manager.go
type WorkersManager interface {
	StopAllWorkers(ctx *Context)
	GetWorker(ctx *Context, id string) (WorkerController, bool)
	RunWorker(ctx *Context, id string, worker WorkerController) error
	StopWorker(ctx *Context, id string) error
	GetWorkerStatus(ctx *Context, id string) (defs.WorkerStatus, error)
	// install workerd bin to workerd bin path, if not specified, use default path /usr/local/bin/workerd
	InstallWorkerd(ctx *Context, url string, path string) (string, error)
}

// services/wg/wireguard_manager.go
type WireGuardManager interface {
	// CreateService 浣跨敤缁欏畾鐨勯厤缃垵濮嬪寲涓€涓柊鐨?WireGuard 骞惰褰?
	// 鏈嶅姟鍒涘缓鍚庝笉浼氳嚜鍔ㄥ惎鍔?
	CreateService(cfg *defs.WireGuardConfig) (WireGuard, error)

	// StartService 鍚姩涔嬪墠娣诲姞鐨?WireGuard
	StartService(interfaceName string) error

	GetService(interfaceName string) (WireGuard, bool)

	StopService(interfaceName string) error

	GetAllServices() []WireGuard

	// RemoveService 鍋滄骞剁Щ闄や竴涓?WireGuard
	RemoveService(interfaceName string) error

	// StopAllServices 鍋滄鎵€鏈?WireGuard
	// 杩斿洖涓€涓?interfaceName 鍒?error 鐨勬槧灏勶紝璁板綍鍋滄澶辫触鐨勬湇鍔?
	StopAllServices() map[string]error

	RestartService(interfaceName string) error

	// Start 鍚姩 WireGuardManager 鏈韩
	Start()
	Stop()
}

type WireGuardDiffPeersResponse struct {
	AddPeers    []*defs.WireGuardPeerConfig
	RemovePeers []*defs.WireGuardPeerConfig
}

// services/wg/wireguard.go
type WireGuard interface {
	Start() error
	Stop() error

	// Peer鐩稿叧
	AddPeer(peer *defs.WireGuardPeerConfig) error
	RemovePeer(peerNameOrPk string) error
	GetPeer(peerNameOrPk string) (*defs.WireGuardPeerConfig, error)
	UpdatePeer(peer *defs.WireGuardPeerConfig) error
	ListPeers() ([]*defs.WireGuardPeerConfig, error)
	PatchPeers(newPeers []*defs.WireGuardPeerConfig) (*WireGuardDiffPeersResponse, error)

	// Interface鐩稿叧
	GetIfceConfig() (*defs.WireGuardConfig, error)
	GetBaseIfceConfig() *defs.WireGuardConfig
	NeedRecreate(newCfg *defs.WireGuardConfig) bool

	// Config鐩稿叧
	GenWGConfig() (string, error) // unimplemented
	GetWGRuntimeInfo() (*pb.WGDeviceRuntimeInfo, error)
	UpdateAdjs(adjs map[uint32]*pb.WireGuardLinks) error
}

type NetworkTopologyCache interface {
	GetRuntimeInfo(wireguardId uint) (*pb.WGDeviceRuntimeInfo, bool)
	SetRuntimeInfo(wireguardId uint, runtimeInfo *pb.WGDeviceRuntimeInfo)
	DeleteRuntimeInfo(wireguardId uint)
	GetLatencyMs(fromWGID, toWGID uint) (uint32, bool)
}
