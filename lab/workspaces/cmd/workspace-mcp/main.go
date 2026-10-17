// Command workspace-mcp serves one session's workspace-VFS MCP surface over HTTP.
//
// It exists so the surface can be probed from a *different process* than the
// writer — the lab's own smoke-alarm, over the wire (register §1, rung 2: a
// `ProtocolMCP` target and a separate process, not this package's own suite).
//
//	workspace-mcp -driver tmpdir -root /tmp/ws42/42 \
//	  -spec ../../sheets/workspaces/minimal.feature -key seed-42 -addr 127.0.0.1:0
//
// With -spec the store is seeded from the spec before it is served: the Gherkin
// is the origin, and the store is its inversion. Without it, an existing session
// is served as it stands.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/james-gibson/lab-safety/workspaces"
)

func main() {
	var (
		driver   = flag.String("driver", "tmpdir", "the host to materialise the session on: tmpdir | sqlite")
		root     = flag.String("root", "", "the session root (a directory, for tmpdir) or the .wsdb path (for sqlite)")
		session  = flag.String("session", "42", "the session id")
		specPath = flag.String("spec", "", "seed the store from this Gherkin spec before serving it")
		key      = flag.String("key", "seed-42", "the session seed: the material the spec's isotope is signed with")
		family   = flag.String("family", "workspace-vfs", "the spec's isotope family")
		version  = flag.Int("version", 1, "the spec's isotope version")
		addr     = flag.String("addr", "127.0.0.1:0", "the listen address")
	)
	flag.Parse()

	if *root == "" {
		fmt.Fprintln(os.Stderr, "workspace-mcp: -root is required")
		os.Exit(2)
	}

	if err := run(*driver, *root, *session, *specPath, *key, *family, *version, *addr); err != nil {
		fmt.Fprintln(os.Stderr, "workspace-mcp:", err)
		os.Exit(1)
	}
}

func run(driver, root, session, specPath, key, family string, version int, addr string) error {
	ctx := context.Background()

	drv, err := open(driver, root)
	if err != nil {
		return err
	}
	defer func() { _ = drv.Close() }()

	s := workspaces.New(drv, session)
	srv := workspaces.NewServer(s)

	if specPath != "" {
		spec, err := workspaces.LoadSpec(specPath, []byte(key), family, version)
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

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// The address may be ":0"; say which port it actually is, so a caller can
	// wait for the surface instead of guessing.
	fmt.Printf("workspace-mcp session=%s driver=%s spec=%s\n", session, drv.Name(), specPath)
	fmt.Printf("listening http://%s/mcp\n", ln.Addr())

	if err := http.Serve(ln, srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
