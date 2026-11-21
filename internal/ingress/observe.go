package ingress

import (
	"encoding/json"
	"fmt"
	"net/http"
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

// Snapshot returns a consistent copy of the counters. A reload shares the
// observer so these numbers survive a config swap.
func (o *Observer) Snapshot() (uint64, map[string]uint64) {
	if o == nil {
		return 0, map[string]uint64{}
	}
	o.mu.Lock()
	quarantined := make(map[string]uint64, len(o.quarantined))
	for reason, count := range o.quarantined {
		quarantined[reason] = count
	}
	forwarded := o.forwarded
	o.mu.Unlock()
	return forwarded, quarantined
}

// Forwarded is the count of accepted forwards the monitor has observed.
func (o *Observer) Forwarded() uint64 {
	forwarded, _ := o.Snapshot()
	return forwarded
}

type statusBody struct {
	Forwarded   uint64            `json:"forwarded"`
	Quarantined map[string]uint64 `json:"quarantined"`
	Ready       bool              `json:"ready"`
}

// ServeStatus publishes the counters as JSON.
func (o *Observer) ServeStatus(ready bool, w http.ResponseWriter, _ *http.Request) {
	forwarded, quarantined := o.Snapshot()
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
