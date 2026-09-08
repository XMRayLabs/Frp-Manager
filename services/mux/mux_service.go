package mux

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type MuxServer interface {
	Run()
	Stop()
}

type muxImpl struct {
	srv *http.Server
	lis net.Listener
	tls bool
}

func NewMux(grpcServer, apiServer http.Handler, lis net.Listener, creds *tls.Config) MuxServer {
	tlsServer := grpcHandlerFunc(grpcServer, apiServer)
	tlsServer.TLSConfig = creds
	return &muxImpl{
		srv: tlsServer,
		lis: lis,
		tls: creds != nil,
	}
}

func (m *muxImpl) Run() {
	if m.lis == nil {
		return
	}
	var err error
	if m.tls {
		err = m.srv.ServeTLS(m.lis, "", "")
	} else {
		err = m.srv.Serve(m.lis)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("frp-manager mux server stopped unexpectedly: %v", err)
	}
}

func (m *muxImpl) Stop() {
	if m.srv == nil || m.lis == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.srv.Shutdown(ctx); err != nil {
		_ = m.srv.Close()
	}
}

func grpcHandlerFunc(grpcServer http.Handler, httpHandler http.Handler) *http.Server {
	return &http.Server{Handler: h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Printf("proto major: %d,  %s , %s\n", r.ProtoMajor, r.RequestURI, r.Header.Get("Content-Type"))
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			httpHandler.ServeHTTP(w, r)
		}
	}), &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
}
