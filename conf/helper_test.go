package conf

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetRPCConnInfo(t *testing.T) {
	parsedUrl, err := url.Parse("grpc://123123:88/123123")
	if err != nil {
	}

	connInfo := ConnInfo{
		Host:   parsedUrl.Host,
		Scheme: Scheme(parsedUrl.Scheme),
	}

	fmt.Printf("%+v", connInfo)
}

func TestPrependExplicitEnvFile(t *testing.T) {
	defaults := []string{".env", "/etc/frpp/.env"}
	got := prependExplicitEnvFile(defaults, ` C:\Program Files\frp-manager\.env `)
	want := []string{`C:\Program Files\frp-manager\.env`, ".env", "/etc/frpp/.env"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected env file order: %v", got)
	}
}

func TestPlatformWorkerdDirUsesAndroidSandboxTemp(t *testing.T) {
	got := platformWorkerdDir("android", "/data/user/0/app/cache", "/tmp/frpp/workerd")
	want := filepath.Join("/data/user/0/app/cache", "frpp", "workerd")
	if got != want {
		t.Fatalf("unexpected Android workerd path: %s", got)
	}
}
