package utils

import "testing"

func TestValidateDownloadURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "public https", rawURL: "https://github.com/example/tool/releases/download/v1/tool"},
		{name: "localhost http", rawURL: "http://localhost:8080/tool"},
		{name: "loopback http", rawURL: "http://127.0.0.1:8080/tool"},
		{name: "public http", rawURL: "http://example.com/tool", wantErr: true},
		{name: "embedded credentials", rawURL: "https://user:password@example.com/tool", wantErr: true},
		{name: "file scheme", rawURL: "file:///tmp/tool", wantErr: true},
		{name: "missing host", rawURL: "https:///tool", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDownloadURL(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDownloadURL(%q) error = %v, wantErr %v", tt.rawURL, err, tt.wantErr)
			}
		})
	}
}
