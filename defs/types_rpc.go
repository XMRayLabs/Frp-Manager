package defs

import (
	"sync"

	"github.com/Sakurame1/frp-manager/pb"
)

type Connector struct {
	Done      chan struct{}
	closeOnce sync.Once
	CliID     string
	Conn      pb.Master_ServerSendServer
	CliType   string
	SendMu    sync.Mutex
}

func (c *Connector) Close() {
	c.closeOnce.Do(func() {
		if c.Done != nil {
			close(c.Done)
		}
	})
}
