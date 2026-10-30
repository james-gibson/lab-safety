// Command pennies-mcp runs the harness surface lab-safety is checked against.
//
//	go run ./lab/pennies/cmd/pennies-mcp --addr=127.0.0.1:18099
//	lab-safety check --probe=http://127.0.0.1:18099/mcp --surface=pennies --gate
//
// The surface is declared in sheets/pennies.md; its price and its enforced gate
// are in policy/skills.yaml. The flags below make it pass or fail on purpose:
//
//	--enforce=false           serve the priced tool without a penny (GATE-OPEN)
//	--register-undeclared     install a tool no sheet declares (PROBE-TOOLS-EXTRA)
//	--unbilled-replay         accept a second redemption and do not record it
//	                          (the unbilled delta; not a tool-list or gate state)
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/james-gibson/lab-safety/lab/pennies"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18099", "listen address")
	manifest := flag.String("manifest", ".well-known/lab-safety.json", "policy manifest served by the handshake")
	enforce := flag.Bool("enforce", true, "require a penny for barista.path")
	undeclared := flag.Bool("register-undeclared", false, "install pennies.bonus, which no sheet declares")
	unbilled := flag.Bool("unbilled-replay", false, "accept a second redemption of a penny without recording it")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen %s: %v\n", *addr, err)
		os.Exit(1)
	}

	srv := &http.Server{
		Handler: pennies.New(pennies.Options{
			Enforce:        *enforce,
			Undeclared:     *undeclared,
			UnbilledReplay: *unbilled,
			ManifestPath:   *manifest,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("pennies harness on http://%s/mcp (enforce=%v, undeclared=%v)", ln.Addr(), *enforce, *undeclared)
	log.Printf("check it with: lab-safety check --probe=http://%s/mcp --surface=pennies --gate", ln.Addr())
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("harness: %v", err)
	}
}
