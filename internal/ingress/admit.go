package ingress

import (
	"crypto/subtle"
	"strings"
)

// Admit is the thin WAN admission check. It runs before resolution so an
// untrusted caller never learns a route. Comparison is constant time.
func Admit(authHeader, token string) bool {
	if token == "" {
		return false
	}
	scheme, presented, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(token)) == 1
}
