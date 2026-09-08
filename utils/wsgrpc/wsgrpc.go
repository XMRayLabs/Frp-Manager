package wsgrpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	WSGrpcError = errors.New("wsgrpc error")
)

type websocketConn struct {
	ws         *websocket.Conn
	readMutex  sync.Mutex
	writeMutex sync.Mutex
	readBuffer bytes.Buffer
}

// Read implements segmented reads for websocket messages.
func (c *websocketConn) Read(p []byte) (int, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	if c.readBuffer.Len() == 0 {
		messageType, data, err := c.ws.ReadMessage()
		if err != nil {
			return 0, errors.Join(err, errors.New("wsgrpc read message error"), WSGrpcError)
		}
		if messageType != websocket.BinaryMessage {
			return 0, errors.Join(fmt.Errorf("unexpected message type: %d", messageType), WSGrpcError)
		}
		c.readBuffer.Write(data)
	}

	if n, err := c.readBuffer.Read(p); err != nil {
		return n, errors.Join(err, WSGrpcError)
	} else {
		return n, nil
	}
}

// Write sends data as a single binary websocket message.
func (c *websocketConn) Write(p []byte) (int, error) {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	err := c.ws.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, errors.Join(err, errors.New("wsgrpc write message error"), WSGrpcError)
	}
	return len(p), nil
}

func (c *websocketConn) Close() error {
	err := c.ws.Close()
	if err != nil {
		return errors.Join(err, errors.New("wsgrpc close error"), WSGrpcError)
	}
	return nil
}

func (c *websocketConn) LocalAddr() net.Addr {
	if conn := c.ws.UnderlyingConn(); conn != nil {
		return conn.LocalAddr()
	}
	return nil
}

func (c *websocketConn) RemoteAddr() net.Addr {
	if conn := c.ws.UnderlyingConn(); conn != nil {
		return conn.RemoteAddr()
	}
	return nil
}

func (c *websocketConn) SetDeadline(t time.Time) error {
	if err := c.ws.SetReadDeadline(t); err != nil {
		return errors.Join(err, errors.New("wsgrpc set read deadline error"), WSGrpcError)
	}
	if err := c.ws.SetWriteDeadline(t); err != nil {
		return errors.Join(err, errors.New("wsgrpc set write deadline error"), WSGrpcError)
	}
	return nil
}

func (c *websocketConn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *websocketConn) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}

type LogInterface interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Tracef(format string, args ...interface{})
}

// WebsocketDialer returns a grpc.WithContextDialer compatible websocket dialer.
func WebsocketDialer(url string, header http.Header, insecure bool, log LogInterface) func(ctx context.Context, addr string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		dialer := websocket.Dialer{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		}
		log.Tracef("dialing websocket server [%s]", url)
		ws, _, err := dialer.DialContext(ctx, url, header)
		if err != nil {
			log.Errorf("wsgrpc dialer error: %v", err)
			return nil, errors.Join(err, errors.New("wsgrpc dialer error"), WSGrpcError)
		}
		log.Tracef("websocket connection connect done")
		return &websocketConn{ws: ws}, nil
	}
}

// WSListener implements net.Listener for websocket-upgraded connections.
type WSListener struct {
	connCh chan net.Conn
	mu     sync.Mutex
	closed bool
	addr   net.Addr
	done   chan struct{}
}

type dummyAddr struct {
	network string
	address string
}

func (d dummyAddr) Network() string {
	return d.network
}

func (d dummyAddr) String() string {
	return d.address
}

func NewWSListener(addr, network string, bufSize int) *WSListener {
	return &WSListener{
		connCh: make(chan net.Conn, bufSize),
		addr:   dummyAddr{network: network, address: addr},
		done:   make(chan struct{}),
	}
}

func (l *WSListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.connCh:
		if !ok {
			return nil, errors.Join(fmt.Errorf("listener closed"), WSGrpcError)
		}
		return conn, nil
	case <-l.done:
		return nil, errors.Join(fmt.Errorf("listener closed"), WSGrpcError)
	}
}

func (l *WSListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	close(l.done)
	close(l.connCh)
	return nil
}

func (l *WSListener) Addr() net.Addr {
	return l.addr
}

// GinWSHandler upgrades HTTP requests to websocket connections and forwards them to the listener.
func GinWSHandler(listener *WSListener, upgrader *websocket.Upgrader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.String(http.StatusInternalServerError, "ws upgrade error: %v", err)
			return
		}
		conn := &websocketConn{ws: ws}
		select {
		case listener.connCh <- conn:
			return
		default:
			ws.Close()
			c.String(http.StatusServiceUnavailable, "connection queue is full")
			return
		}
	}
}
