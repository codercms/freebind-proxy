package proxy

import (
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Option interface {
	apply(srv *Server)
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

type ListenAddrOption struct {
	addr string
}

func (o *ListenAddrOption) apply(srv *Server) {
	srv.listenAddr = o.addr
}

func WithListenAddr(addr string) *ListenAddrOption {
	return &ListenAddrOption{addr}
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

type WithLoggerOption struct {
	logger *zap.Logger
}

func (o *WithLoggerOption) apply(srv *Server) {
	srv.logger = o.logger
}

func WithLogger(logger *zap.Logger) *WithLoggerOption {
	return &WithLoggerOption{logger}
}
