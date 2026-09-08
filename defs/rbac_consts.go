package defs

type RBACAction string

const (
	RBACActionCreate RBACAction = "create"
	RBACActionRead   RBACAction = "read"
	RBACActionUpdate RBACAction = "update"
	RBACActionDelete RBACAction = "delete"
	RBACActionShare  RBACAction = "share"
	RBACActionView   RBACAction = "view"
	RBACActionEdit   RBACAction = "edit"
)

type RBACObj string

const (
	RBACObjServer RBACObj = "server"
	RBACObjClient RBACObj = "client"
	RBACObjWorker RBACObj = "worker"
	RBACObjUser   RBACObj = "user"
	RBACObjGroup  RBACObj = "group"
	RBACObjAPI    RBACObj = "api"
)

type RBACSubject string

const (
	RBACSubjectUser  RBACSubject = "user"
	RBACSubjectGroup RBACSubject = "group"
	RBACSubjectToken RBACSubject = "token"
)

type RBACDomain string

const (
	RBACDomainTenant RBACDomain = "tenant"
)

type APIPermission struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

const (
	TokenPayloadKey_Permissions = "permissions"
)
