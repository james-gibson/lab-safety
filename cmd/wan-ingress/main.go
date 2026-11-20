// Command wan-ingress is a single static binary: a thin WAN-facing front that
// resolves arbitrary inbound names by agreement and forwards them only through
// a signed border. See internal/ingress/README.md and
// sheets/ingress/wan-ingress.feature.
//
//	wan-ingress -agreements=agreements.json -token=... -border-key=... -addr=:8443
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/james-gibson/lab-safety/internal/ingress"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8443", "bind address (0.0.0.0 is a bind, never a destination)")
	agreementsPath := flag.String("agreements", "", "path to the JSON file of agreed mappings")
	token := flag.String("token", "", "bearer token required for WAN admission")
	borderKey := flag.String("border-key", "", "HMAC key for the signed border envelope")
	registry := flag.String("smoke-alarm", "", "smoke-alarm registry URL to register by name")
	flag.Parse()

	resolver := ingress.NewEmptyResolver()
	if *agreementsPath != "" {
		file, err := os.Open(*agreementsPath)
		if err != nil {
			log.Fatalf("open agreements: %v", err)
		}
		resolver, err = ingress.LoadAgreements(file)
		_ = file.Close()
		if err != nil {
			log.Fatalf("load agreements: %v", err)
		}
	}

	server := &ingress.Server{
		Resolver:  resolver,
		Forwarder: &ingress.Forwarder{BorderKey: []byte(*borderKey)},
		Token:     *token,
		Observer:  ingress.NewObserver(),
	}

	if *registry != "" {
		endpoint := "http://" + *addr
		if strings.HasPrefix(*addr, ":") {
			endpoint = "http://127.0.0.1" + *addr
		}
		status, body, err := ingress.Register(context.Background(), *registry, "wan-ingress", endpoint)
		if err != nil {
			log.Printf("smoke-alarm registration failed: %v", err)
		} else {
			log.Printf("registered name=wan-ingress endpoint=%s status=%d body=%s", endpoint, status, strings.TrimSpace(string(body)))
		}
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("wan-ingress listening on %s", *addr)
	log.Fatal(httpServer.ListenAndServe())
}
