package proxy

import (
	"context"
	"fmt"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"go.uber.org/zap"
	"net"
	"net/http"
	"time"
)

type AuthCheckFunc func(usr, passwd string) bool

type Server struct {
	dFactory DialerFactoryIface

	authFunc AuthCheckFunc

	httpSrv *http.Server

	listenAddr string

	gracefulShutdownTimeout time.Duration

	logger *zap.Logger
}

func MakeServer(dFactory DialerFactoryIface, options ...Option) *Server {
	srv := &Server{dFactory: dFactory}

	for _, option := range options {
		option.apply(srv)
	}

	if srv.gracefulShutdownTimeout < 1 {
		srv.gracefulShutdownTimeout = 5 * time.Second
	}

	if len(srv.listenAddr) == 0 {
		srv.listenAddr = ":8080"
	}
	if srv.httpSrv == nil {
		srv.httpSrv = &http.Server{}
	}

	if srv.logger == nil {
		srv.logger = zap.NewNop()
	}

	return srv
}

var sockHookInstalled = struct{}{}

func (s *Server) hookCtx(dFactory DialerFactoryIface, ctx *goproxy.ProxyCtx) {
	// Do not reinstall hook
	if ctx.UserData == sockHookInstalled {
		return
	}

	dialer := dFactory.GetDialer()

	// Assign new transport to proxy for each request
	ctx.Proxy.Tr = ctx.Proxy.Tr.Clone()
	ctx.Proxy.Tr.Dial = dialer.Dial
	ctx.Proxy.Tr.DialContext = dialer.DialContext
	ctx.UserData = sockHookInstalled

	if dialer.LocalAddr != nil && s.logger.Level().Enabled(zap.DebugLevel) {
		s.logger.Debug("Select IP to perform request", zap.String("ip", dialer.LocalAddr.String()))
	}
}

type proxyLogger struct {
	logger *zap.Logger
}

func (p *proxyLogger) Printf(format string, v ...interface{}) {
	p.logger.Sugar().Debugf(format, v...)
}

func (s *Server) Run(ctx context.Context) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = s.logger.Level().Enabled(zap.DebugLevel)
	proxy.Logger = &proxyLogger{logger: s.logger}

	onAllReqs := proxy.OnRequest()

	if s.authFunc != nil {
		var realm string

		// Register auth for each request
		onAllReqs.Do(auth.Basic("", s.authFunc))

		// Also register auth for proxy CONNECT
		basicConn := auth.BasicConnect(realm, s.authFunc)
		onAllReqs.HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			action, host := basicConn.HandleConnect(host, ctx)

			// Prevent goproxy behavior when it receives non-nil action it stops next handlers invocation
			if action == goproxy.OkConnect {
				return nil, host
			}

			return action, host
		})
	}

	onAllReqs.HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if s.logger.Level().Enabled(zap.DebugLevel) {
			s.logger.Debug("Handle CONNECT func", zap.String("host", host))
		}

		s.hookCtx(s.dFactory, ctx)

		return nil, host
	})
	onAllReqs.DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if s.logger.Level().Enabled(zap.DebugLevel) {
			s.logger.Debug("Handle req func", zap.String("url", req.URL.String()))
		}

		s.hookCtx(s.dFactory, ctx)

		return req, nil
	})

	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server addr: %w", err)
	}

	s.logger.Info("Listening on address", zap.String("addr", s.listenAddr))

	errChan := make(chan error, 1)

	s.httpSrv.Addr = s.listenAddr
	s.httpSrv.Handler = proxy

	go func() {
		defer close(errChan)

		errChan <- s.httpSrv.Serve(listener)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.gracefulShutdownTimeout)
		defer cancel()

		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to gracefully shutdown server: %w", err)
		}

		return nil
	}
}
