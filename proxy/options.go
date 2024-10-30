package proxy

import (
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Option interface {
	apply(srv *Server)
}

type ListenAddrOption struct {
	addr string
}

func (o *ListenAddrOption) apply(srv *Server) {
	srv.listenAddr = o.addr
}

func WithListenAddr(addr string) *ListenAddrOption {
	return &ListenAddrOption{addr}
}

type WithLoggerOption struct {
	logger *zap.Logger
}

func (o *WithLoggerOption) apply(srv *Server) {
	srv.logger = o.logger
}

func WithLogger(logger *zap.Logger) *WithLoggerOption {
	return &WithLoggerOption{logger}
}

type AuthFuncOption struct {
	authFunc AuthCheckFunc
}

func (o *AuthFuncOption) apply(srv *Server) {
	srv.authFunc = o.authFunc
}

func WithAuthFunc(authFunc AuthCheckFunc) *AuthFuncOption {
	return &AuthFuncOption{authFunc}
}

type WithHttpServerOption struct {
	httpServer *http.Server
}

func (o *WithHttpServerOption) apply(srv *Server) {
	srv.httpSrv = o.httpServer
}

func WithHttpServer(httpServer *http.Server) *WithHttpServerOption {
	return &WithHttpServerOption{httpServer}
}

type GracefulShutdownTimeoutOption struct {
	timeout time.Duration
}

func (o *GracefulShutdownTimeoutOption) apply(srv *Server) {
	srv.gracefulShutdownTimeout = o.timeout
}

func WithGracefulShutdownTimeout(timeout time.Duration) *GracefulShutdownTimeoutOption {
	return &GracefulShutdownTimeoutOption{timeout}
}

// WithHttpTransportOption allows to set preconfigured HTTP transport when proxy serves non HTTPS (HTTP only) requests
type WithHttpTransportOption struct {
	transport *http.Transport
}

func (o *WithHttpTransportOption) apply(srv *Server) {
	srv.baseHttpTransport = o.transport
}

func WithHttpTransport(transport *http.Transport) *WithHttpTransportOption {
	return &WithHttpTransportOption{transport}
}
