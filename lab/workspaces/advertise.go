package workspaces

import (
	"context"
	"fmt"

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
