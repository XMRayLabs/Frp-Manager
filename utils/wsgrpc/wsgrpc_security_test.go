package wsgrpc

import (
	"encoding/binary"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestOversizeFrameRejectedBeforePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	listener := NewWSListener("audit", "tcp", 1)
	defer listener.Close()
	router := gin.New()
	router.GET("/ws", GinWSHandler(listener, &websocket.Upgrader{}))
	server := httptest.NewServer(router)
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	// A masked binary frame declares >32 MiB but sends no payload. The reader
	// must reject its header instead of waiting for or allocating the body.
	header := make([]byte, 14)
	header[0] = 0x82
	header[1] = 0xff
	binary.BigEndian.PutUint64(header[2:10], (32<<20)+1)
	if _, err = ws.UnderlyingConn().Write(header); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Read(make([]byte, 1))
	if err == nil || !strings.Contains(err.Error(), "read limit") {
		t.Fatalf("frame not rejected by length: %v", err)
	}
}

func TestNormalFramesRemainCompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	listener := NewWSListener("audit", "tcp", 1)
	defer listener.Close()
	router := gin.New()
	router.GET("/ws", GinWSHandler(listener, &websocket.Upgrader{}))
	server := httptest.NewServer(router)
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = ws.WriteMessage(websocket.BinaryMessage, []byte("normal grpc bytes")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("normal grpc bytes"))
	_, err = io.ReadFull(conn, got)
	if err != nil || string(got) != "normal grpc bytes" {
		t.Fatal(string(got), err)
	}
}
