package api

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAPIServerStopsGracefully(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := NewApiService(listener, router, true)
	done := make(chan struct{})
	go func() {
		server.Run()
		close(done)
	}()

	response, err := http.Get("http://" + listener.Addr().String() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	server.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("API server did not stop")
	}
}

func TestAPIServerAllowsMissingListener(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewApiService(nil, gin.New(), true)
	server.Run()
	server.Stop()
}
