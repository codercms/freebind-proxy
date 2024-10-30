package proxy

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func MakeProxyAuthMiddleware(next http.Handler, checkFunc AuthCheckFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(r, checkFunc) {
			w.Header().Set("Proxy-Authenticate", "Basic realm=\"Restricted\"")
			http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func checkAuth(r *http.Request, checkFunc AuthCheckFunc) bool {
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

	return checkFunc(string(payload[:colDelim]), string(payload[colDelim+1:]))
}
