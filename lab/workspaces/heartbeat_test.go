package workspaces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A session's registry entry is a heartbeat: the name must be renewed while the
// surface serves, or smoke-alarm sweeps it (smoke-alarm#35).
func TestKeepRegisteredRenewsTheName(t *testing.T) {
	var posts atomic.Int32

	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/isotope/register" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		posts.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "workspace-42", "trust_rung": 2,
		})
	}))
	defer reg.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errs := KeepRegistered(ctx, reg.URL, SessionName("42"), "127.0.0.1:9999", 10*time.Millisecond)

	deadline := time.After(2 * time.Second)
	for posts.Load() < 2 {
		select {
		case <-deadline:
			cancel()
			t.Fatalf("renewed %d time(s), want at least 2", posts.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()

	// The channel closes when the heartbeat stops; a caller waits on it before
	// deregistering so no beat can re-add the name afterwards.
	select {
	case _, ok := <-errs:
		if ok {
			t.Fatal("unexpected renewal error")
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not stop after cancellation")
	}
}

func TestDeregisterRemovesTheSession(t *testing.T) {
	var deletes atomic.Int32

	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/isotope/workspace-42" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deletes.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer reg.Close()

	if err := Deregister(context.Background(), reg.URL, SessionName("42")); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if got := deletes.Load(); got != 1 {
		t.Fatalf("deregistered %d time(s), want 1", got)
	}
}

// An entry that already expired is already in the desired state.
func TestDeregisterToleratesMissingEntry(t *testing.T) {
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer reg.Close()

	if err := Deregister(context.Background(), reg.URL, SessionName("42")); err != nil {
		t.Fatalf("a 404 is a success, got %v", err)
	}
}

func TestDeregisterWithoutARegistryIsANoOp(t *testing.T) {
	if err := Deregister(context.Background(), "", SessionName("42")); err != nil {
		t.Fatalf("no registry configured: %v", err)
	}
}
