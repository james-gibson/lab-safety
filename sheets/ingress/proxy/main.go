// Command invite-proxy stands in for a peer cluster's egress or alarms in the
// wan-ingress harness.
//
// A visitor never carries an ingress URL. It carries an invite — an isotope
// name (ultimately a seed-42 pointer resolved out of band) — and this proxy
// resolves that name from the lab's isotope registry before forwarding the
// call. The wan-ingress binary stays deliberately unaware of addressing: it
// still only sees bearer admission, a name to parse, and an agreed border.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const inviteHeader = "X-Invite"

type isotopeRecord struct {
	Name         string    `json:"name"`
	Endpoint     string    `json:"endpoint"`
	RegisteredAt time.Time `json:"registered_at"`
}

// resolve turns an invite (an isotope name) into the current binding of that
// name. With several records for one name the most recently registered wins —
// the registry's own last-writer-wins rule — so a restarted ingress is found
// without the caller knowing its port.
func resolve(ctx context.Context, registryURL, name string, client *http.Client) (string, error) {
	endpoint := strings.TrimRight(registryURL, "/") + "/isotope/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build registry request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("query registry: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %s", resp.Status)
	}

	var records []isotopeRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return "", fmt.Errorf("decode registry: %w", err)
	}
	var best isotopeRecord
	for _, rec := range records {
		if rec.Name == name && rec.Endpoint != "" && rec.RegisteredAt.After(best.RegisteredAt) {
			best = rec
		}
	}
	if best.Endpoint == "" {
		return "", fmt.Errorf("no isotope named %q in %s", name, registryURL)
	}
	return strings.TrimRight(best.Endpoint, "/"), nil
}

func singleJoining(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return base
	}
	return base + "/" + path
}

// newHandler is the whole proxy: resolve the invite, then forward. A caller's
// own Authorization header is preserved so rate-limit and auth red-team tests
// still reach the ingress; the configured token is the invite's default grant.
func newHandler(registryURL, defaultName, token string, client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		invite := r.Header.Get(inviteHeader)
		if invite == "" {
			invite = defaultName
		}
		endpoint, err := resolve(r.Context(), registryURL, invite, client)
		if err != nil {
			log.Printf("resolve invite %q: %v", invite, err)
			http.Error(w, "invite not resolvable", http.StatusBadGateway)
			return
		}
		target, err := url.Parse(endpoint)
		if err != nil || target.Host == "" {
			log.Printf("invalid endpoint %q for invite %q: %v", endpoint, invite, err)
			http.Error(w, "invite not resolvable", http.StatusBadGateway)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Director = func(out *http.Request) {
			out.URL.Scheme = target.Scheme
			out.URL.Host = target.Host
			out.URL.Path = singleJoining(target.Path, out.URL.Path)
			out.Host = target.Host
			out.Header = out.Header.Clone()
			if out.Header.Get("Authorization") == "" && token != "" {
				out.Header.Set("Authorization", "Bearer "+token)
			}
			out.Header.Del(inviteHeader)
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("forward invite %q: %v", invite, err)
			http.Error(w, "egress unavailable", http.StatusBadGateway)
		}
		proxy.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address (loopback by default)")
	registry := flag.String("registry", "", "isotope registry URL (smoke-alarm)")
	name := flag.String("name", "wan-ingress", "default invite to resolve when X-Invite is absent")
	token := flag.String("token", "", "bearer token injected when the caller supplies none")
	flag.Parse()
	if *registry == "" {
		log.Fatal("-registry is required")
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	log.Printf("invite-proxy listening addr=%s registry=%s name=%s", ln.Addr(), *registry, *name)
	log.Fatal(http.Serve(ln, newHandler(*registry, *name, *token, client)))
}
