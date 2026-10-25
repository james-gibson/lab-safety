package workspaces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	errs := KeepRegistered(ctx, reg.URL, SessionName("42"), "127.0.0.1:9999", References{}, 10*time.Millisecond)

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

// identityBody is the registration as it arrives: the shared fields plus the
// project identity a session publishes.
type identityBody struct {
	Name         string `json:"name"`
	ProjectRoute string `json:"project_route"`
	Contract     string `json:"contract"`
}

// A session publishes its project route and the spec's contract id, so a
// dependent can resolve the project instead of remembering the endpoint.
func TestRegisterWithReferencesSpeaksIdentity(t *testing.T) {
	var got identityBody

	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode registration: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": got.Name, "trust_rung": 2,
		})
	}))
	defer reg.Close()

	refs := References{ProjectRoute: "01JLAB", Contract: "workspace-vfs-v1:abc123"}
	if _, err := RegisterWithReferences(context.Background(), reg.URL, SessionName("42"), "127.0.0.1:9999", refs); err != nil {
		t.Fatalf("RegisterWithReferences: %v", err)
	}

	if got.Name != "workspace-42" || got.ProjectRoute != "01JLAB" || got.Contract != "workspace-vfs-v1:abc123" {
		t.Fatalf("registration = %+v, want the session's route and contract", got)
	}
}

// Every heartbeat resends the identity, so a renewal cannot erase it.
func TestKeepRegisteredResendsIdentity(t *testing.T) {
	bodies := make(chan identityBody, 8)

	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body identityBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode registration: %v", err)
		}
		bodies <- body
		_ = json.NewEncoder(w).Encode(map[string]any{"name": body.Name, "trust_rung": 2})
	}))
	defer reg.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errs := KeepRegistered(ctx, reg.URL, SessionName("42"), "127.0.0.1:9999",
		References{ProjectRoute: "01JLAB", Contract: "workspace-vfs-v1:abc123"}, 10*time.Millisecond)

	select {
	case body := <-bodies:
		if body.ProjectRoute != "01JLAB" || body.Contract != "workspace-vfs-v1:abc123" {
			t.Fatalf("renewal lost identity: %+v", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no renewal arrived")
	}
	cancel()
	for range errs {
	}
}

func TestRouteFromSeed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.json")

	if err := os.WriteFile(path, []byte(`{"seed":{"id":"LEGACY"},"project":{"name":"lab","route":"01JROUTE"}}`), 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if got := RouteFromSeed(path); got != "01JROUTE" {
		t.Fatalf("route = %q, want project.route", got)
	}

	if err := os.WriteFile(path, []byte(`{"seed":{"id":"LEGACY"},"project":{"name":"lab"}}`), 0o600); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if got := RouteFromSeed(path); got != "LEGACY" {
		t.Fatalf("route = %q, want the legacy seed.id", got)
	}

	if got := RouteFromSeed(filepath.Join(dir, "missing.json")); got != "" {
		t.Fatalf("missing seed returned %q, want empty", got)
	}
}
