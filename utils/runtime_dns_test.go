package utils

import (
	"reflect"
	"testing"
)

func TestRuntimeDNSServerAddresses(t *testing.T) {
	got := runtimeDNSServerAddresses(" 192.0.2.53,2001:db8::53,192.0.2.53,[2001:db8::54]:5353 ")
	want := []string{
		"192.0.2.53:53",
		"[2001:db8::53]:53",
		"[2001:db8::54]:5353",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected DNS server addresses: %v", got)
	}
}
