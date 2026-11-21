package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HeartbeatInterval is well inside smoke-alarm's isotope TTL (5 minutes): a
// registration is a lease, and a name that stops renewing stops resolving.
const HeartbeatInterval = time.Minute

// Register puts the front into the lab's isotope registry by name, in the same
// shape lab/workspaces uses. The registry — not this process — judges the rung.
func Register(ctx context.Context, registryURL, name, endpoint string) (int, []byte, error) {
	body, err := json.Marshal(map[string]string{
		"name": name, "role": "standalone", "endpoint": endpoint, "protocol": "http",
	})
	if err != nil {
		return 0, nil, err
	}
	target := strings.TrimRight(registryURL, "/") + "/isotope/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, answer, err
}

// KeepRegistered renews the name until ctx is canceled. A failed beat is not
// fatal: the process still serves and the next beat retries, so a transient
// registry outage cannot take the front down. The returned channel closes when
// the heartbeat stops; a caller drains it before deregistering so no beat can
// re-add the name afterwards.
func KeepRegistered(ctx context.Context, registryURL, name, endpoint string, interval time.Duration) <-chan error {
	if interval <= 0 {
		interval = HeartbeatInterval
	}
	errs := make(chan error, 16)
	go func() {
		defer close(errs)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status, _, err := Register(ctx, registryURL, name, endpoint)
				if err == nil && (status < 200 || status >= 300) {
					err = fmt.Errorf("registry answered %d", status)
				}
				if err != nil {
					if ctx.Err() != nil {
						return // shutdown canceled an in-flight beat; not a failure
					}
					select {
					case errs <- err:
					default: // an undrained channel must not stall the beat
					}
				}
			}
		}
	}()
	return errs
}

// Deregister asks the registry to give up the name on a clean shutdown. A
// missing entry (404) is already the desired end state. smoke-alarm#112 tracks
// adding the route; until then this is best-effort and the TTL is authoritative.
func Deregister(ctx context.Context, registryURL, name string) error {
	if registryURL == "" {
		return nil
	}
	target := strings.TrimRight(registryURL, "/") + "/isotope/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, http.NoBody)
	if err != nil {
		return fmt.Errorf("deregister %s: %w", name, err)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("deregister %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("deregister %s: registry answered %d", name, resp.StatusCode)
	}
	return nil
}
