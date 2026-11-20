package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Observer counts decisions. It is evidence for another process to read, not
// a claim that the node is healthy (lab/README.md: no node reports on itself).
type Observer struct {
	mu          sync.Mutex
	forwarded   uint64
	quarantined map[string]uint64
}

// NewObserver returns an empty counter set.
func NewObserver() *Observer {
	return &Observer{quarantined: make(map[string]uint64)}
}

// Record adds one decision to the counters.
func (o *Observer) Record(d Decision) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.quarantined == nil {
		o.quarantined = make(map[string]uint64)
	}
	if d.Decision == "forward" {
		o.forwarded++
		return
	}
	if d.Reason != "" {
		o.quarantined[d.Reason]++
	}
}

type statusBody struct {
	Forwarded   uint64            `json:"forwarded"`
	Quarantined map[string]uint64 `json:"quarantined"`
	Ready       bool              `json:"ready"`
}

// ServeStatus publishes the counters as JSON.
func (o *Observer) ServeStatus(ready bool, w http.ResponseWriter, _ *http.Request) {
	o.mu.Lock()
	quarantined := make(map[string]uint64, len(o.quarantined))
	for reason, count := range o.quarantined {
		quarantined[reason] = count
	}
	forwarded := o.forwarded
	o.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusBody{
		Forwarded: forwarded, Quarantined: quarantined, Ready: ready,
	})
}

// ServeHealthz answers without any configuration.
func ServeHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprint(w, "ok")
}

// ServeReadyz reports whether agreements, a token and a border key are present.
func ServeReadyz(ready bool, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, "not ready")
		return
	}
	_, _ = fmt.Fprint(w, "ready")
}

// Register puts the front into the lab's isotope registry by name, in the same
// shape lab/workspaces uses. The registry — not this process — judges the rung.
func Register(ctx context.Context, registryURL, name, endpoint string) (int, []byte, error) {
	body, err := json.Marshal(map[string]string{
		"name": name, "role": "standalone", "endpoint": endpoint, "protocol": "http",
	})
	if err != nil {
		return 0, nil, err
	}
	url := strings.TrimRight(registryURL, "/") + "/isotope/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return resp.StatusCode, answer, err
}
