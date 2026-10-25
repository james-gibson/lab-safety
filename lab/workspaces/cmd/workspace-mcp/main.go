// Command workspace-mcp serves one session's workspace-VFS MCP surface over HTTP,
// and registers the session in the lab's own isotope registry.
//
// It exists so the surface can be probed from a *different process* than the
// writer — the lab's own smoke-alarm, over the wire (register §1, rung 2: a
// `ProtocolMCP` target and a separate process, not this package's own suite).
//
// With -smoke-alarm the session becomes a *node* the lab can reflect: it registers
// by name (POST /isotope/register) so no caller has to be told its URL. That is
// the frame's reading — the session is a user in the root's namespace, resolved by
// the root, not an author of it.
//
//	workspace-mcp -driver tmpdir -root /tmp/ws42/42 \
//	  -spec ../../sheets/workspaces/minimal.feature -key seed-42 \
//	  -smoke-alarm http://127.0.0.1:6100
//
// With -spec the store is seeded from the spec before it is served: the Gherkin is
// the origin, and the store is its inversion.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/james-gibson/lab-safety/workspaces"
)

type options struct {
	driver     string
	root       string
	session    string
	specPath   string
	key        string
	family     string
	version    int
	addr       string
	smokeAlarm string
}

func main() {
	var o options
	flag.StringVar(&o.driver, "driver", "tmpdir", "the host to materialise the session on: tmpdir | sqlite")
	flag.StringVar(&o.root, "root", "", "the session root (a directory, for tmpdir) or the .wsdb path (for sqlite)")
	flag.StringVar(&o.session, "session", "42", "the session id")
	flag.StringVar(&o.specPath, "spec", "", "seed the store from this Gherkin spec before serving it")
	flag.StringVar(&o.key, "key", "seed-42", "the session seed: the material the spec's isotope is signed with")
	flag.StringVar(&o.family, "family", "workspace-vfs", "the spec's isotope family")
	flag.IntVar(&o.version, "version", 1, "the spec's isotope version")
	flag.StringVar(&o.addr, "addr", "127.0.0.1:0", "the listen address")
	flag.StringVar(&o.smokeAlarm, "smoke-alarm", "", "register the session with this smoke-alarm isotope registry, so it is reached by name")
	flag.Parse()

	if o.root == "" {
		fmt.Fprintln(os.Stderr, "workspace-mcp: -root is required")
		os.Exit(2)
	}

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "workspace-mcp:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	// SIGTERM/SIGINT cancels the heartbeat and shuts the surface down, so the
	// session can remove its registry name instead of leaving it to expire.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	drv, err := open(o.driver, o.root)
	if err != nil {
		return err
	}
	defer func() { _ = drv.Close() }()

	s := workspaces.New(drv, o.session)
	srv := workspaces.NewServer(s)

	if o.specPath != "" {
		spec, err := workspaces.LoadSpec(o.specPath, []byte(o.key), o.family, o.version)
		if err != nil {
			return err
		}
		if err := workspaces.Seed(ctx, s, spec); err != nil {
			return err
		}
		if err := workspaces.BindTools(srv, spec); err != nil {
			return err
		}
	}

	ln, err := net.Listen("tcp", o.addr)
	if err != nil {
		return err
	}

	// The address may be ":0"; say which port it actually is, so a caller can wait
	// for the surface instead of guessing.
	fmt.Printf("workspace-mcp session=%s driver=%s spec=%s\n", o.session, drv.Name(), o.specPath)
	fmt.Printf("listening http://%s/mcp\n", ln.Addr())

	// The session registers itself, by name, with the lab's registry. It is not
	// fatal if this fails — the surface still serves — but it must not be silent:
	// an unregistered session is not reachable by name.
	//
	// A name is a heartbeat, not a label: smoke-alarm sweeps a registration it
	// has not heard from (smoke-alarm#35), so the session renews while it serves
	// and deregisters when it stops. Without the renewal its name would expire
	// under a live session; without the deregistration a clean stop would leave
	// the name behind until the TTL passed.
	if o.smokeAlarm != "" {
		name := workspaces.SessionName(o.session)
		rec, err := workspaces.Register(ctx, o.smokeAlarm, name, ln.Addr().String())
		if err != nil {
			fmt.Printf("registration failed name=%s registry=%s: %v\n", name, o.smokeAlarm, err)
		} else {
			fmt.Printf("registered name=%s endpoint=%s rung=%d (%s)\n", rec.Name, rec.Endpoint, rec.TrustRung, rec.RungName)
		}

		renewCtx, stopRenew := context.WithCancel(ctx)
		heartbeatDone := make(chan struct{})
		go func() {
			defer close(heartbeatDone)
			for err := range workspaces.KeepRegistered(renewCtx, o.smokeAlarm, name, ln.Addr().String(), workspaces.HeartbeatInterval) {
				fmt.Printf("registration renewal failed name=%s: %v\n", name, err)
			}
		}()
		defer func() {
			// Stop renewing before deleting, or a beat in flight could put the
			// name straight back after deregistration.
			stopRenew()
			<-heartbeatDone

			deregCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := workspaces.Deregister(deregCtx, o.smokeAlarm, name); err != nil {
				fmt.Printf("deregistration failed name=%s: %v\n", name, err)
			}
		}()
	}

	httpSrv := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func open(driver, root string) (workspaces.Driver, error) {
	switch driver {
	case "tmpdir":
		return workspaces.NewTmpDir(root)
	case "sqlite":
		return workspaces.NewSQLite(root)
	default:
		return nil, fmt.Errorf("unknown driver %q (want tmpdir | sqlite)", driver)
	}
}
