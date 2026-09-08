package mux

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestMuxServerStopsGracefully(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	server := NewMux(handler, handler, listener, nil)
	done := make(chan struct{})
	go func() {
		server.Run()
		close(done)
	}()

	response, err := http.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	server.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("mux server did not stop")
	}
}

func TestMuxServerAllowsDisabledListener(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := NewMux(handler, handler, nil, nil)
	server.Run()
	server.Stop()
}
