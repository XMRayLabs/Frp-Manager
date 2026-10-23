package client

import (
	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	plugin "github.com/fatedier/frp/pkg/plugin/client"
)

func init() { plugin.Register(websiteauth.PluginType, websiteauth.NewPlugin) }
