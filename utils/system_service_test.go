package utils

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestRedactServiceArgs(t *testing.T) {
	input := []string{"client", "-s", "secret-one", "--secret=secret-two", "-i", "client-1", "-j", "token-one", "--join-token=token-two"}
	got := redactServiceArgs(input)
	want := []string{"client", "-s", "[redacted]", "--secret=[redacted]", "-i", "client-1", "-j", "[redacted]", "--join-token=[redacted]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected redacted args: %v", got)
	}
	if input[2] != "secret-one" {
		t.Fatal("redaction mutated the original argument slice")
	}
}

func TestServiceEnvironment(t *testing.T) {
	executable := filepath.Join(`C:\Program Files`, "frp-manager", "frpp.exe")
	env := serviceEnvironment(executable)
	if runtime.GOOS != "windows" {
		if env != nil {
			t.Fatalf("unexpected service environment: %v", env)
		}
		return
	}
	if env["FRP_MANAGER_ENV_FILE"] != filepath.Join(filepath.Dir(executable), ".env") {
		t.Fatalf("unexpected service env path: %v", env)
	}
	if env["LOGGER_FILE"] != filepath.Join(filepath.Dir(executable), "service.log") {
		t.Fatalf("unexpected service log path: %v", env)
	}
}
