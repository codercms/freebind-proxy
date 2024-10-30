package proxy

import (
	"context"
	"net"
	"net/http"
)

type ctxDialerKeyType struct{}

var ctxDialerKey ctxDialerKeyType

type ctxDialer struct {
	dialer    *net.Dialer
	transport *http.Transport
}

func setCtxDialer(ctx context.Context, d *ctxDialer) context.Context {
	return context.WithValue(ctx, ctxDialerKey, d)
}
func getCtxDialer(ctx context.Context) *ctxDialer {
	return ctx.Value(ctxDialerKey).(*ctxDialer)
}
