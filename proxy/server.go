package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
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

	srvCtx context.Context

	baseHttpTransport *http.Transport
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

	if srv.baseHttpTransport == nil {
		srv.baseHttpTransport = http.DefaultTransport.(*http.Transport)
	}

	return srv
}

func (s *Server) configureHttpServer() {
	var httpHandler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			s.handleConnect(w, r)
		} else {
			s.handleHTTP(w, r)
		}
	})

	if s.authFunc != nil {
		httpHandler = MakeProxyAuthMiddleware(httpHandler, s.authFunc)
	}

	s.httpSrv.Addr = s.listenAddr
	s.httpSrv.Handler = httpHandler

	// Monitor connections state
	s.httpSrv.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			s.logger.Debug("Incoming HTTP connection",
				zap.String("remote", conn.RemoteAddr().String()),
			)
		case http.StateClosed:
			s.logger.Debug("Closed HTTP connection",
				zap.String("remote", conn.RemoteAddr().String()),
			)
		default:
			return
		}
	}

	// Set conn context for not CONNECT requests
	s.httpSrv.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		ctx = setCtxDialer(ctx, &ctxDialer{})

		return ctx
	}
}

// Run starts proxy HTTP server
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to bind HTTP server addr: %w", err)
	}

	s.logger.Info("Listening on address", zap.String("addr", listener.Addr().String()))

	s.srvCtx = ctx

	s.configureHttpServer()

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		errChan <- s.httpSrv.Serve(listener)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.logger.Info("Shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.gracefulShutdownTimeout)
		defer cancel()

		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("Failed to shutdown HTTP server", zap.Error(err))

			return fmt.Errorf("failed to gracefully shutdown server: %w", err)
		}

		s.logger.Info("HTTP server shutdown completed")

		return nil
	}
}

// checkAuthorization checks if the provided credentials are valid
func (s *Server) checkAuthorization(r *http.Request) bool {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	// Expected authorization header format: "Basic <base64-encoded-credentials>"
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}

	// Decode the base64 credentials
	payload, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return false
	}

	colDelim := strings.IndexByte(string(payload), ':')
	if colDelim < 0 || len(payload) < colDelim+2 {
		return false
	}

	if !s.authFunc(string(payload[:colDelim]), string(payload[colDelim+1:])) {
		s.logger.Warn("Bad auth attempt", zap.String("remote", r.RemoteAddr))
	}

	return true
}

// handleConnect handles the CONNECT (tunnelled HTTP) method
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	dialer := s.dFactory.GetDialer()
	if dialer.LocalAddr != nil && s.logger.Level().Enabled(zap.DebugLevel) {
		s.logger.Debug("Selected IP to perform request",
			zap.String("remote", r.RemoteAddr),
			zap.String("dialerIp", dialer.LocalAddr.String()),
		)
	}

	destConn, err := dialer.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		s.logger.Warn("Failed to dial host",
			zap.String("host", r.Host),
			zap.String("remote", r.RemoteAddr),
			zap.Error(err),
		)

		http.Error(w, "Failed to connect to the destination", http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		s.logger.Error("Hijacking connection is not supported",
			zap.String("remote", r.RemoteAddr),
		)

		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	// Take over the connection
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		s.logger.Error("Failed to hijack connection",
			zap.String("remote", r.RemoteAddr),
		)

		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Don't block too long when trying to respond to the client
	_ = clientConn.SetWriteDeadline(time.Now().Add(time.Second * 5))

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		s.logger.Warn("Failed to send 200 Connection established to client",
			zap.String("remote", r.RemoteAddr),
			zap.Error(err),
		)
		return
	}

	bufSize := 32 * 1024
	clientBuf := make([]byte, bufSize)
	dstBuf := make([]byte, bufSize)

	clientConnDeadlineRw := &NetConnTimeoutReadWriter{
		conn:    clientConn,
		timeout: time.Second * 5,
	}
	dstConnDeadlineRw := &NetConnTimeoutReadWriter{
		conn:    destConn,
		timeout: time.Second * 5,
	}

	proxyCtx, proxyCancel := context.WithCancel(context.Background())
	defer proxyCancel()

	// Client -> Target
	go func() {
		defer proxyCancel()

		_, err := CopyBufferWithTimeout(dstConnDeadlineRw, clientConnDeadlineRw, clientBuf)
		if err != nil {
			s.logger.Warn("Err while transferring data from client to destination",
				zap.String("remote", r.RemoteAddr),
				zap.String("dst", r.Host),
				zap.Error(err),
			)
		}
	}()

	// Target -> Client
	go func() {
		defer proxyCancel()

		_, err := CopyBufferWithTimeout(clientConnDeadlineRw, dstConnDeadlineRw, dstBuf)
		if err != nil {
			var netOpErr *net.OpError
			// Ignore error when client connection has been closed
			if errors.As(err, &netOpErr) && netOpErr.Err != nil && strings.Contains(netOpErr.Err.Error(), "use of closed network connection") {
				return
			}

			s.logger.Warn("Err while transferring data from destination to client",
				zap.String("remote", r.RemoteAddr),
				zap.String("dst", r.Host),
				zap.Error(err),
			)
		}
	}()

	select {
	case <-s.srvCtx.Done():
	case <-proxyCtx.Done():
		s.logger.Debug("Client conn closed",
			zap.String("remote", clientConn.RemoteAddr().String()),
			zap.String("host", r.Host),
		)
	}
}

// handleHTTP handles regular (not tunneled) HTTP requests
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	ctxDialer := getCtxDialer(r.Context())
	if ctxDialer.dialer == nil {
		ctxDialer.dialer = s.dFactory.GetDialer()

		ctxDialer.transport = s.baseHttpTransport.Clone()
		ctxDialer.transport.Dial = ctxDialer.dialer.Dial
		ctxDialer.transport.DialContext = ctxDialer.dialer.DialContext

		if ctxDialer.dialer.LocalAddr != nil && s.logger.Level().Enabled(zap.DebugLevel) {
			s.logger.Debug("Selected IP to perform request",
				zap.String("remote", r.RemoteAddr),
				zap.String("dialerIp", ctxDialer.dialer.LocalAddr.String()),
			)
		}
	}

	// If no Accept-Encoding header exists, Transport will add the headers it can accept
	// and would wrap the response body with the relevant reader.
	r.Header.Del("Accept-Encoding")
	// curl can add that, see
	// https://jdebp.eu./FGA/web-proxy-connection-header.html
	r.Header.Del("Proxy-Connection")
	r.Header.Del("Proxy-Authenticate")
	r.Header.Del("Proxy-Authorization")
	// Connection, Authenticate and Authorization are single hop Header:
	// http://www.w3.org/Protocols/rfc2616/rfc2616.txt
	// 14.10 Connection
	//   The Connection general-header field allows the sender to specify
	//   options that are desired for that particular connection and MUST NOT
	//   be communicated by proxies over further connections.

	// When server reads http request it sets req.Close to true if
	// "Connection" header contains "close".
	// https://github.com/golang/go/blob/master/src/net/http/request.go#L1080
	// Later, transfer.go adds "Connection: close" back when req.Close is true
	// https://github.com/golang/go/blob/master/src/net/http/transfer.go#L275
	// That's why tests that checks "Connection: close" removal fail
	if r.Header.Get("Connection") == "close" {
		r.Close = false
	}
	r.Header.Del("Connection")

	resp, err := ctxDialer.transport.RoundTrip(r)
	if err != nil {
		s.logger.Warn("Failed to perform HTTP request",
			zap.String("remote", r.RemoteAddr),
			zap.String("host", r.Host),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)

		http.Error(w, "Failed to perform HTTP request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Do not send connection close header
	if resp.Header.Get("Connection") != "upgrade" {
		resp.Header.Del("Connection")
	}

	// Copy response headers to proxy response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Error("Failed to copy HTTP response body",
			zap.String("remote", r.RemoteAddr),
			zap.Error(err),
		)
	}
}
