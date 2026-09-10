package server

import (
	"encoding/json"
	"github.com/Sakurame1/frp-manager/models"
)

func serverConfigForUser(u models.UserInfo, s *models.ServerEntity) string {
	if u.IsAdmin() {
		return string(s.ConfigContent)
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(s.ConfigContent, &config) != nil {
		return ""
	}
	safe := map[string]json.RawMessage{}
	for _, key := range []string{"bindPort", "kcpBindPort", "quicBindPort", "vhostHTTPPort", "vhostHTTPSPort", "subDomainHost", "allowPorts"} {
		if value, ok := config[key]; ok {
			safe[key] = value
		}
	}
	data, err := json.Marshal(safe)
	if err != nil {
		return ""
	}
	return string(data)
}
func serverSecretForUser(u models.UserInfo, s *models.ServerEntity) string {
	if u.IsAdmin() {
		return s.ConnectSecret
	}
	return ""
}
