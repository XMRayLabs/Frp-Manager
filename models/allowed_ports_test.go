package models

import "testing"

func TestAllowedPorts(t *testing.T) {
	for _, bad := range []string{"0", "65536", "64000-62000", "62000-", "1-2-3", "1,2", "+80", "a"} {
		if _, err := NormalizeAllowedPorts(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, tc := range []struct{ input, want string }{{"", ""}, {" 62000 - 63999 ", "62000-63999"}, {"0080", "80"}} {
		got, err := NormalizeAllowedPorts(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("%q: %q %v", tc.input, got, err)
		}
	}
	server := []byte(`{"vhostHTTPPort":62000,"vhostHTTPSPort":443,"tcpmuxHTTPConnectPort":63000}`)
	for _, tc := range []struct {
		cfg   string
		valid bool
	}{
		{`{"type":"tcp","remotePort":62000,"plugin":{"type":"http_proxy"}}`, true},
		{`{"type":"tcp","remotePort":63999,"plugin":{"type":"socks5"}}`, true},
		{`{"type":"udp","remotePort":63000}`, true},
		{`{"type":"tcp","remotePort":61999}`, false},
		{`{"type":"tcp","remotePort":64000}`, false},
		{`{"type":"tcp","remotePort":0}`, false},
		{`{"type":"http"}`, true}, {`{"type":"https"}`, false}, {`{"type":"tcpmux"}`, true}, {`{"type":"stcp"}`, true},
	} {
		if err := ValidateProxyAllowedPorts("62000-63999", []byte(tc.cfg), server); (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.cfg, err)
		}
	}
	if err := ValidateProxyAllowedPorts("", []byte(`{"type":"tcp","remotePort":0}`), nil); err != nil {
		t.Fatal(err)
	}
}
