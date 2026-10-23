package frpauth

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	plugin "github.com/fatedier/frp/pkg/plugin/server"
)

// Decode uses FRP's operation-specific wire format. Server identity is supplied
// by the authenticated daemon, never by the plugin HTTP caller.
func Decode(body io.Reader) (*pb.FRPAuthRequest, error) {
	var input struct {
		Op      string          `json:"op"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&input); err != nil {
		return nil, err
	}
	req := &pb.FRPAuthRequest{Operation: input.Op}
	switch input.Op {
	case "Login":
		var c plugin.LoginContent
		if err := json.Unmarshal(input.Content, &c); err != nil {
			return nil, err
		}
		req.User, req.Token = c.User, c.Metas[defs.FRPAuthTokenKey]
	case "NewProxy":
		var c plugin.NewProxyContent
		if err := json.Unmarshal(input.Content, &c); err != nil {
			return nil, err
		}
		if c.RemotePort < 0 || c.RemotePort > 65535 {
			return nil, fmt.Errorf("invalid remote port")
		}
		req.User, req.Token = c.User.User, c.User.Metas[defs.FRPAuthTokenKey]
		req.ProxyType, req.RemotePort = c.ProxyType, int32(c.RemotePort)
		if c.ProxyType == "http" || c.ProxyType == "https" {
			req.NewProxy, _ = json.Marshal(c.NewProxy)
		}
	case "Ping", "NewWorkConn":
		var c struct {
			User plugin.UserInfo `json:"user"`
		}
		if err := json.Unmarshal(input.Content, &c); err != nil {
			return nil, err
		}
		req.User, req.Token = c.User.User, c.User.Metas[defs.FRPAuthTokenKey]
	default:
		return nil, fmt.Errorf("unsupported authentication operation")
	}
	if req.User == "" || req.Token == "" {
		return nil, fmt.Errorf("missing FRP credentials")
	}
	return req, nil
}
