// Command healthcheck is a tiny stdlib HTTP probe for distroless containers.
// It exists so a hardened image can carry a real Docker HEALTHCHECK without
// shipping curl, wget or a shell.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8443/healthz", "health endpoint")
	timeout := flag.Duration("timeout", 2*time.Second, "request timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}
	resp, err := client.Get(*url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		fmt.Fprintf(os.Stderr, "health status %s\n", resp.Status)
		os.Exit(1)
	}
}
