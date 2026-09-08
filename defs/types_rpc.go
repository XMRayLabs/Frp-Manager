package defs

import (
	"sync"

	"github.com/Sakurame1/frp-manager/pb"
)

type Connector struct {
	CliID   string
	Conn    pb.Master_ServerSendServer
	CliType string
	SendMu  sync.Mutex
}
