package transport

import (
	"net/http"
	"strings"

	"github.com/Neuxbane/NeuXbaneProtocol/nxp/abi"
)

// AuthCookieNames lists the cookie names that the built-in transports treat as
// bearer credentials when no Authorization header is present. The first cookie
// found (in this order) wins. Applications that use a custom cookie name should
// add it here so that ingress identity extraction and folder guards agree.
var AuthCookieNames = []string{
	"xm_session",
	"token",
	"auth_token",
	"session",
	"tg_token",
}

// ExtractIdentity derives an *abi.Identity from an HTTP request using the
// Authorization: Bearer header first, then the well-known auth cookies. It
// returns nil when no credential is present.
//
// This is shared by the REST and WebSocket transports so that a caller who
// authenticates during a WebSocket upgrade is seen as the same identity on
// every subsequent frame.
func ExtractIdentity(r *http.Request) *abi.Identity {
	if r == nil {
		return nil
	}

	token := ""
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if token == "" {
		for _, name := range AuthCookieNames {
			if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
				token = cookie.Value
				break
			}
		}
	}
	if token == "" {
		return nil
	}

	return &abi.Identity{
		Subject: token,
		Raw:     token,
	}
}
