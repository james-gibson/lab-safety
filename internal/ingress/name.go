// Package ingress is the WAN front described by sheets/ingress/wan-ingress.feature.
//
// The resolver is a total function: it accepts any string and returns either a
// canonical lookup key or a typed refusal. It never derives an address from the
// input — addresses and namespaces come only from an operator-signed agreement.
package ingress

import (
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ErrMalformed is the typed refusal for input the parser will not interpret.
var ErrMalformed = errors.New("malformed name")

// MaxNameBytes bounds a name. 253 is the longest DNS label-bearing name the
// resolver will consider; longer input is refused, never truncated.
const MaxNameBytes = 253

// ParseName maps any string to a canonical lookup key or ErrMalformed. It
// percent-decodes exactly once, rejects control characters and path syntax,
// and lower-cases the result. No address, port or namespace is produced here.
func ParseName(raw string) (string, error) {
	if raw == "" || len(raw) > MaxNameBytes || !utf8.ValidString(raw) || hasControl(raw) {
		return "", ErrMalformed
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil || decoded == "" || len(decoded) > MaxNameBytes ||
		!utf8.ValidString(decoded) || hasControl(decoded) {
		return "", ErrMalformed
	}
	if strings.ContainsAny(decoded, "/\\") || strings.Contains(decoded, "..") {
		return "", ErrMalformed
	}

	// Trim to a fixed point: "0 ." must not become "0 " here and "0" on the
	// second pass. A canonical name is the same name when read again.
	canonical := decoded
	for {
		next := strings.TrimSuffix(strings.TrimSpace(canonical), ".")
		if next == canonical {
			break
		}
		canonical = next
	}
	canonical = strings.ToLower(canonical)
	if canonical == "" || len(canonical) > MaxNameBytes || hasControl(canonical) {
		return "", ErrMalformed
	}
	return canonical, nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
