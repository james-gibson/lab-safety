package ingress

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

// ErrBorderUnconfigured is the typed refusal when no signed border is available.
var ErrBorderUnconfigured = errors.New("border key or border address not configured")

const maxForwardBody = 1 << 20

// Forwarder signs and sends one request to an agreed border. It never accepts
// a destination from the caller.
type Forwarder struct {
	Client    *http.Client
	BorderKey []byte
}

// Ready reports whether a signed border is configured.
func (f *Forwarder) Ready() bool {
	return f != nil && len(f.BorderKey) > 0
}

// Forward copies the route's address into a URL, signs the envelope and posts
// the body. Any destination supplied by the caller is ignored.
func (f *Forwarder) Forward(ctx context.Context, route Route, name string, body []byte) (int, error) {
	if !f.Ready() || route.Address == "" {
		return 0, ErrBorderUnconfigured
	}
	if len(body) > maxForwardBody {
		return 0, fmt.Errorf("body exceeds %d bytes", maxForwardBody)
	}

	u, err := url.Parse(route.Address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return 0, ErrBorderUnconfigured
	}
	cleanName, err := ParseName(name)
	if err != nil {
		return 0, err
	}
	u.Path = path.Join(u.Path, "namespace", route.Namespace, "name", cleanName)
	u.RawQuery = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	digest := sha256.Sum256(body)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, f.BorderKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s\n%s",
		http.MethodPost, u.EscapedPath(), route.Namespace, hex.EncodeToString(digest[:]), timestamp)
	req.Header.Set("X-Lab-Signature", hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Lab-Namespace", route.Namespace)
	req.Header.Set("X-Lab-Timestamp", timestamp)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, nil
}
