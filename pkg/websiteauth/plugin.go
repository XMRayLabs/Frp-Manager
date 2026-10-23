package websiteauth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	v1 "github.com/fatedier/frp/pkg/config/v1"
	plugin "github.com/fatedier/frp/pkg/plugin/client"
	netpkg "github.com/fatedier/frp/pkg/util/net"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type websitePlugin struct {
	listener  *connectionListener
	server    *http.Server
	transport *http.Transport
	mu        sync.Mutex
	closed    bool
	conns     map[*trackedConn]struct{}
}

func NewPlugin(_ plugin.PluginContext, raw v1.ClientPluginOptions) (plugin.Plugin, error) {
	opts, ok := raw.(*Options)
	if !ok || opts == nil {
		return nil, fmt.Errorf("invalid website authentication options")
	}
	options := opts.Clone().(*Options)
	if err := options.Normalize(); err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext, MaxIdleConns: 64, MaxIdleConnsPerHost: 16, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: 60 * time.Second}
	reverse := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL.Scheme = "http"
			p.Out.URL.Host = options.LocalAddr
			// Outer credentials belong to this gateway, never to the destination website.
			p.Out.Header.Del("Authorization")
			p.Out.Header.Del("Proxy-Authorization")
		},
		Transport: transport, ErrorLog: log.New(io.Discard, "", 0),
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Website upstream unavailable", http.StatusBadGateway)
		},
	}
	handler := newAuthHandler(options, reverse)
	p := &websitePlugin{listener: newConnectionListener(), transport: transport, conns: map[*trackedConn]struct{}{}}
	p.server = &http.Server{Handler: handler, ReadHeaderTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024, ErrorLog: log.New(io.Discard, "", 0)}
	go func() { _ = p.server.Serve(p.listener) }()
	return p, nil
}
func (p *websitePlugin) Name() string { return PluginType }

type trackedConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *trackedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.onClose); return err }
func (p *websitePlugin) Handle(_ context.Context, info *plugin.ConnectionInfo) {
	conn := &trackedConn{Conn: netpkg.WrapReadWriteCloserToConn(info.Conn, info.UnderlyingConn)}
	conn.onClose = func() { p.mu.Lock(); delete(p.conns, conn); p.mu.Unlock() }
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = conn.Close()
		return
	}
	p.conns[conn] = struct{}{}
	p.mu.Unlock()
	if err := p.listener.PutConn(conn); err != nil {
		_ = conn.Close()
	}
}
func (p *websitePlugin) Close() error {
	p.mu.Lock()
	p.closed = true
	conns := make([]*trackedConn, 0, len(p.conns))
	for c := range p.conns {
		conns = append(conns, c)
	}
	p.mu.Unlock()
	err := p.server.Close()
	_ = p.listener.Close()
	// http.Server.Close does not close hijacked WebSockets; revoke those as well.
	for _, c := range conns {
		_ = c.Close()
	}
	p.transport.CloseIdleConnections()
	return err
}

func newAuthHandler(options *Options, next http.Handler) http.Handler {
	hashes := map[string]string{}
	for _, a := range options.Accounts {
		hashes[a.Username] = a.PasswordHash
	}
	// Successful credentials are cached briefly to avoid bcrypt on every image or
	// WebSocket request. Cache is bounded and discarded on every config change.
	var mu sync.Mutex
	cache := map[[32]byte]time.Time{}
	limiter := rate.NewLimiter(5, 10)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			http.Error(w, "Method not allowed", 405)
			return
		}
		username, password, ok := r.BasicAuth()
		key := sha256.Sum256([]byte(username + "\x00" + password))
		now := time.Now()
		mu.Lock()
		until := cache[key]
		mu.Unlock()
		accepted := ok && now.Before(until)
		if !accepted && ok && len(username) <= 64 && len(password) <= 72 {
			if hash, exists := hashes[username]; exists {
				if !limiter.Allow() {
					w.Header().Set("Retry-After", "1")
					http.Error(w, "Too many authentication attempts", 429)
					return
				}
				accepted = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
				if accepted {
					mu.Lock()
					if len(cache) >= 128 {
						clear(cache)
					}
					cache[key] = now.Add(5 * time.Minute)
					mu.Unlock()
				}
			}
		}
		if !accepted {
			w.Header().Set("WWW-Authenticate", `Basic realm="Website", charset="UTF-8"`)
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// A closeable queue avoids closing a channel while a work connection is being queued.
type connectionListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newConnectionListener() *connectionListener {
	return &connectionListener{conns: make(chan net.Conn, 64), done: make(chan struct{})}
}
func (l *connectionListener) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, net.ErrClosed
	case c := <-l.conns:
		return c, nil
	}
}
func (l *connectionListener) PutConn(c net.Conn) error {
	select {
	case <-l.done:
		return net.ErrClosed
	default:
	}
	select {
	case <-l.done:
		return net.ErrClosed
	case l.conns <- c:
		return nil
	}
}
func (l *connectionListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *connectionListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }
