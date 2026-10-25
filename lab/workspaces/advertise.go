package workspaces

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/james-gibson/isotope"
)

// The session as a node in the lab's registry.
//
// A session is not a special case of the lab: it registers where every other node
// registers — smoke-alarm's isotope registry (`POST /isotope/register`, through
// `isotope.Client`) — and is then resolved by **name**, out of band, by whatever
// needs to reach it. So no caller spells out a session's URL: the harness asks
// the registry, exactly as adhd's own peer discovery does
// (`adhd/internal/headless/isotope_discovery.go` → `DiscoverIsotopes`, which reads
// the registry rather than mDNS).
//
// This is the frame's reading of the plane (§2): the session is a **user in the
// root's namespace**, and the root reflects it. The session never holds the
// root's material — the wire contract is the prober's, and the trust rung is the
// registry's judgement, not ours.
//
// mDNS (`isotope.Advertise`) is deliberately not done here. adhd's peer view is
// built from the registry, not from mDNS; and announcing under adhd's
// `_adhd-isotope._tcp` would claim to *be* an adhd. When a second host needs LAN
// discovery, that is the call — with its own service type.

// SessionName is the name a session registers under. It matches the surface's own
// `serverInfo.name` (protocol.go), so the registry and the wire agree on what the
// session is called.
func SessionName(session string) string { return "workspace-" + session }

// Register puts a session's surface into the lab's isotope registry and returns
// the record the registry assigned — including its trust rung.
func Register(ctx context.Context, registryURL, name, endpoint string) (*isotope.Record, error) {
	if registryURL == "" {
		return nil, fmt.Errorf("no registry configured")
	}
	return isotope.NewClient(registryURL).Register(ctx, isotope.Registration{
		Name:     name,
		Role:     "standalone",
		Endpoint: endpoint,
		Protocol: "mcp",
	})
}

// HeartbeatInterval is how often a session renews its registry entry. It stays
// well inside smoke-alarm's isotope TTL (5m by default): the registry treats a
// registration as a heartbeat and sweeps a name it has not heard from, so a
// session that stops renewing stops resolving by name (smoke-alarm#35).
const HeartbeatInterval = time.Minute

// KeepRegistered renews a session's registry entry every interval until ctx is
// canceled, so the name stays resolvable while the surface is up. It returns a
// channel of renewal errors (closed when the heartbeat stops); the caller may
// drain it for visibility or ignore it — a renewal failure is not fatal, because
// the surface still serves and the next beat retries.
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
				if _, err := Register(ctx, registryURL, name, endpoint); err != nil {
					select {
					case errs <- err:
					default: // a caller that is not reading must not stall the beat
					}
				}
			}
		}
	}()
	return errs
}

// Deregister removes a session's registry entry so a node that shuts down
// cleanly does not leave a name behind. A missing entry (404) is success: the
// name is gone, which is the desired end state.
func Deregister(ctx context.Context, registryURL, name string) error {
	if registryURL == "" {
		return nil
	}

	endpoint := strings.TrimRight(registryURL, "/") + "/isotope/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("deregister %s: build request: %w", name, err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req) //nolint:gosec // G704: the registry is the operator-configured smoke-alarm endpoint
	if err != nil {
		return fmt.Errorf("deregister %s: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("deregister %s: registry answered %s", name, resp.Status)
	}
	return nil
}
