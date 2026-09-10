package client

import "encoding/json"

// Shared members may configure tunnels, but never receive the device's common
// authentication, TLS key paths, transport proxy credentials or log paths.
func sharedClientConfig(raw string) *string {
	if raw == "" {
		return nil
	}
	var cfg map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &cfg) != nil {
		return nil
	}
	safe := map[string]json.RawMessage{}
	for _, key := range []string{"serverAddr", "serverPort", "proxies", "visitors"} {
		if v, ok := cfg[key]; ok {
			safe[key] = v
		}
	}
	data, err := json.Marshal(safe)
	if err != nil {
		return nil
	}
	s := string(data)
	return &s
}
