package conf

import (
	"strings"
	"testing"
)

func TestPrintConfigRedactsEnrollmentToken(t *testing.T) {
	cfg := Config{}
	cfg.Client.JoinToken = "private-enrollment-token"
	if strings.Contains(cfg.PrintStr(), cfg.Client.JoinToken) {
		t.Fatal("enrollment token leaked")
	}
	if cfg.Client.JoinToken != "private-enrollment-token" {
		t.Fatal("printing mutated configuration")
	}
}
