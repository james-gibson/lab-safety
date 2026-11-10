// Command workspace-agent serves one session's workspace VFS as an ACP v1 agent
// over stdio, so an editor can drive the virtual tree while it is being built.
//
// It is a debug console, not the lab's front door: there is no model, no
// permission flow, and no safety apparatus. A prompt is a command (see
// `help`), executed against the session's store and streamed back as a tool
// call.
//
//	workspace-agent -driver tmpdir -root /tmp/ws42/42
//	workspace-agent -driver sqlite -root /tmp/ws42/42.wsdb \
//	  -spec ../../sheets/workspaces/minimal.feature -key seed-42
//
// Stdout carries the ACP protocol; everything else goes to stderr.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	acp "github.com/ironpark/acp-go"
	acp1 "github.com/ironpark/acp-go/acp1"

	"github.com/james-gibson/lab-safety/agent"
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
	flag.StringVar(&o.addr, "addr", "", "serve the agent's own MCP surface on this HTTP address (e.g. 127.0.0.1:0); empty = stdio only")
	flag.StringVar(&o.smokeAlarm, "smoke-alarm", "", "register the agent's MCP surface with this smoke-alarm isotope registry, so it is reached by name")
	flag.Parse()

	if o.root == "" {
		fmt.Fprintln(os.Stderr, "workspace-agent: -root is required")
		os.Exit(2)
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "workspace-agent:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	ctx := context.Background()

	drv, err := open(o.driver, o.root)
	if err != nil {
		return err
	}
	defer func() { _ = drv.Close() }()

	s := workspaces.New(drv, o.session)

	toolNames := make([]string, 0)
	var spec workspaces.SpecRef
	haveSpec := false
	if o.specPath != "" {
		spec, err = workspaces.LoadSpec(o.specPath, []byte(o.key), o.family, o.version)
		if err != nil {
			return err
		}
		if err := workspaces.Seed(ctx, s, spec); err != nil {
			return err
		}
		want, err := spec.Expectation()
		if err != nil {
			return err
		}
		toolNames = want.Tools
		haveSpec = true
	} else if ok, err := s.Present(ctx); err != nil {
		return err
	} else if !ok {
		if err := s.Init(ctx); err != nil {
			return err
		}
	}

	// The stand-in for an executable "@ file, as in the workspace surface.
	echo := func(in []byte) []byte { return in }

	log.SetOutput(os.Stderr)
	log.Printf("workspace-agent session=%s driver=%s spec=%s", o.session, drv.Name(), o.specPath)

	// The agent's own MCP surface (adhd#63): the same store, served by *this*
	// process. A watcher that reaches it by name is watching the agent's
	// liveness, not a separate surface process that can outlive it.
	if o.addr != "" || o.smokeAlarm != "" {
		srv := workspaces.NewNamedServer(s, workspaces.AgentName(o.session))
		if haveSpec {
			if err := workspaces.BindTools(srv, spec); err != nil {
				return err
			}
		}

		addr := o.addr
		if addr == "" {
			addr = "127.0.0.1:0"
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		log.Printf("workspace-agent mcp listening http://%s/mcp", ln.Addr())

		if o.smokeAlarm != "" {
			name := workspaces.AgentName(o.session)
			rec, err := workspaces.Register(ctx, o.smokeAlarm, name, ln.Addr().String())
			if err != nil {
				log.Printf("workspace-agent registration failed name=%s registry=%s: %v", name, o.smokeAlarm, err)
			} else {
				log.Printf("workspace-agent registered name=%s endpoint=%s rung=%d (%s)", rec.Name, rec.Endpoint, rec.TrustRung, rec.RungName)
			}
		}

		go func() {
			if err := http.Serve(ln, srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("workspace-agent mcp server: %v", err)
			}
		}()
	}

	conn := acp1.NewAgentSideConnection(
		func(c *acp1.AgentSideConnection) acp1.Agent {
			a := agent.New(c, s, drv.Name())
			for _, name := range toolNames {
				a.RegisterTool(name, echo)
			}
			return a
		},
		acp.NewStdioTransport(os.Stdin, os.Stdout),
	)
	if err := conn.Start(ctx); err != nil {
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
