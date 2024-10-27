package proxy

import (
	"context"
	"fmt"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strings"
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

type roundTripper struct {
	transport *http.Transport
}

func (rt *roundTripper) RoundTrip(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Response, error) {
	return rt.transport.RoundTrip(req)
}

type proxyLogger struct {
	logger *zap.SugaredLogger
}

func (p *proxyLogger) Printf(format string, v ...any) {
	if (len(format) >= 4 && format[:4] == "WARN") || (len(format) >= 11 && strings.Contains(format[:11], "WARN")) {
		p.logger.Warnf(format, v...)
		return
	}

	p.logger.Debugf(format, v...)
}

const proxyCtxKey = "proxy"

func (s *Server) installRoundTripper(ctx *goproxy.ProxyCtx) {
	if ctx.RoundTripper != nil {
		return
	}

	transport := ctx.Proxy.Tr.Clone()

	dialer := s.dFactory.GetDialer()

	transport.Dial = dialer.Dial
	transport.DialContext = dialer.DialContext

	ctx.RoundTripper = &roundTripper{transport: transport}
	ctx.UserData = dialer

	if ctx.Req != nil {
		ctx.Req = ctx.Req.WithContext(context.WithValue(ctx.Req.Context(), proxyCtxKey, ctx))
	}

	if dialer.LocalAddr != nil && s.logger.Level().Enabled(zap.DebugLevel) {
		s.logger.Debug("Selected IP to perform request", zap.String("ip", dialer.LocalAddr.String()))
	}
}

func (s *Server) Run(ctx context.Context) error {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = s.logger.Level().Enabled(zap.DebugLevel)
	proxy.Logger = &proxyLogger{logger: s.logger.Sugar()}

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

		s.installRoundTripper(ctx)

		return nil, host
	})
	onAllReqs.DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if s.logger.Level().Enabled(zap.DebugLevel) {
			s.logger.Debug("Handle req func", zap.String("url", req.URL.String()))
		}

		s.installRoundTripper(ctx)

		return req, nil
	})

	proxy.ConnectDialWithReq = func(req *http.Request, network string, addr string) (net.Conn, error) {
		if proxyConn, ok := req.Context().Value(proxyCtxKey).(*goproxy.ProxyCtx); ok {
			if dialer, ok := proxyConn.UserData.(*net.Dialer); ok {
				return dialer.Dial(network, addr)
			}
		}

		return net.Dial(network, addr)
	}

	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server addr: %w", err)
	}

	s.logger.Info("Listening on address", zap.String("addr", listener.Addr().String()))

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
