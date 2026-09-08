package shared

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

type nodeIdentity struct {
	API    string       `json:"api"`
	Role   defs.AppRole `json:"role"`
	Name   string       `json:"name"`
	ID     string       `json:"id,omitempty"`
	Secret string       `json:"secret,omitempty"`
}

func writeNodeIdentity(path string, identity nodeIdentity) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".node-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func autoJoinNode(cfg conf.Config, args CommonArgs, role defs.AppRole) (conf.Config, error) {
	if role != defs.AppRole_Client && role != defs.AppRole_Server {
		return cfg, nil
	}
	if cfg.Client.ID != "" && cfg.Client.Secret != "" {
		return cfg, nil
	}
	if args.JoinToken == nil || *args.JoinToken == "" {
		args.JoinToken = &cfg.Client.JoinToken
	}
	api := strings.TrimRight(conf.GetAPIURL(cfg), "/")
	parsed, err := url.Parse(api)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return cfg, errors.New("panel API URL must be an http(s) URL without credentials, query or fragment")
	}
	cfg.Client.APIUrl = api
	if cfg.Client.RPCUrl == "" {
		if strings.HasPrefix(api, "https://") {
			cfg.Client.RPCUrl = "wss://" + strings.TrimPrefix(api, "https://")
		} else if strings.HasPrefix(api, "http://") {
			cfg.Client.RPCUrl = "ws://" + strings.TrimPrefix(api, "http://")
		}
	}
	args.ApiUrl, args.RpcUrl = &cfg.Client.APIUrl, &cfg.Client.RPCUrl
	dir, err := os.UserConfigDir()
	if err != nil {
		return cfg, err
	}
	key := sha256.Sum256([]byte(fmt.Sprintf("%s|%v|%s", api, role, lo.FromPtr(args.ClientID))))
	path := filepath.Join(dir, "frp-manager", fmt.Sprintf("node-%x.json", key[:12]))
	identity := nodeIdentity{API: api, Role: role}
	ephemeral := role == defs.AppRole_Client && lo.FromPtr(args.Ephemeral)
	if !ephemeral {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &identity); err != nil {
				return cfg, fmt.Errorf("read node identity %s: %w", path, err)
			}
			if identity.API != api || identity.Role != role || identity.Name == "" {
				return cfg, errors.New("node identity does not match panel or role")
			}
			if identity.ID != "" && identity.Secret != "" {
				cfg.Client.ID, cfg.Client.Secret = identity.ID, identity.Secret
				return cfg, nil
			}
		} else if !os.IsNotExist(err) {
			return cfg, err
		}
	}
	if err := checkPullParams(args); err != nil {
		return cfg, err
	}
	if identity.Name == "" {
		identity.Name = lo.FromPtr(args.ClientID)
		if identity.Name == "" {
			identity.Name = utils.MakeClientIDPermited(utils.GetHostnameWithIP()) + "-" + uuid.NewString()[:8]
		}
		// Save the enrollment name before the request so retries use the same node.
		if !ephemeral {
			if err := writeNodeIdentity(path, identity); err != nil {
				return cfg, fmt.Errorf("save node identity: %w", err)
			}
		}
	}
	args.ClientID = &identity.Name
	var node *pb.Client
	if role == defs.AppRole_Server {
		node, err = rpc.JoinServer(cfg, identity.Name, lo.FromPtr(args.JoinToken))
	} else {
		node, err = JoinMaster(cfg, args)
	}
	if err != nil {
		return cfg, err
	}
	if node == nil || node.GetId() == "" || node.GetSecret() == "" {
		return cfg, errors.New("panel returned empty node credentials")
	}
	identity.ID, identity.Secret = node.GetId(), node.GetSecret()
	if !ephemeral {
		if err := writeNodeIdentity(path, identity); err != nil {
			return cfg, fmt.Errorf("save registered node identity: %w", err)
		}
	}
	cfg.Client.ID, cfg.Client.Secret = identity.ID, identity.Secret
	return cfg, nil
}
