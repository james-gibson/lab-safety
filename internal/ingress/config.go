package ingress

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
)

// Config is the operator-owned policy. It is deliberately small: an agreement
// file, the admission token, the border key, and the registry to register with.
// A config file lets SIGHUP swap all of it in place as traffic is added; flags
// override the file so an operator can still launch with one command.
type Config struct {
	Addr       string `json:"addr,omitempty"`
	Agreements string `json:"agreements,omitempty"`
	Token      string `json:"token,omitempty"`
	BorderKey  string `json:"border_key,omitempty"`
	SmokeAlarm string `json:"smoke_alarm,omitempty"`
}

// LoadConfigFile reads a JSON config. Unknown keys are refused: a typo must not
// silently leave the ingress defensive when an operator believes it is serving.
func LoadConfigFile(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = file.Close() }()

	var cfg Config
	dec := json.NewDecoder(io.LimitReader(file, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// NewServer builds a server from a config. The observer is passed in so a
// reload keeps the counters the monitor has already seen; a new observer would
// make the light forget every quarantine it had reported.
func NewServer(cfg Config, observer *Observer) (*Server, error) {
	resolver := NewEmptyResolver()
	if cfg.Agreements != "" {
		file, err := os.Open(cfg.Agreements)
		if err != nil {
			return nil, fmt.Errorf("open agreements: %w", err)
		}
		resolver, err = LoadAgreements(file)
		_ = file.Close()
		if err != nil {
			return nil, err
		}
	}
	if observer == nil {
		observer = NewObserver()
	}
	srv := &Server{
		Resolver:  resolver,
		Forwarder: &Forwarder{BorderKey: []byte(cfg.BorderKey)},
		Token:     cfg.Token,
		Observer:  observer,
	}
	srv.Handler() // build the mux once, at swap time
	return srv, nil
}

// Runtime is the atomic config switch. SIGHUP builds a new Server off to the
// side and swaps it in; a request in flight finishes on the old policy, and the
// next one sees the new one. An invalid config never replaces a good one.
type Runtime struct {
	current atomic.Pointer[Server]
}

// Swap installs a server as the current policy.
func (rt *Runtime) Swap(srv *Server) {
	if srv == nil {
		return
	}
	srv.Handler()
	rt.current.Store(srv)
}

// Current returns the server serving right now.
func (rt *Runtime) Current() *Server {
	if rt == nil {
		return nil
	}
	return rt.current.Load()
}

// ServeHTTP dispatches to the current policy. A nil runtime is not ready; it
// never falls through to an empty, permissive handler.
func (rt *Runtime) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv := rt.Current()
	if srv == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	srv.Handler().ServeHTTP(w, r)
}
