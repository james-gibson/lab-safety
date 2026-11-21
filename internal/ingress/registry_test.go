package ingress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A name is a lease: the front renews it while it serves, and a beat that
// fails is retried rather than killing the process.
func TestKeepRegisteredRenewsTheName(t *testing.T) {
	var posts atomic.Int32
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/isotope/register" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode registration: %v", err)
		}
		if body["name"] != "wan-ingress" || body["protocol"] != "http" {
			t.Errorf("registration = %+v", body)
		}
		posts.Add(1)
		_, _ = w.Write([]byte(`{"name":"wan-ingress"}`))
	}))
	defer reg.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errs := KeepRegistered(ctx, reg.URL, "wan-ingress", "http://127.0.0.1:8443", 5*time.Millisecond)
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
	select {
	case _, ok := <-errs:
		if ok {
			t.Fatal("unexpected heartbeat error")
		}
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not stop after cancellation")
	}
}

func TestDeregisterGivesUpTheName(t *testing.T) {
	var deletes atomic.Int32
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/isotope/wan-ingress" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deletes.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer reg.Close()

	if err := Deregister(context.Background(), reg.URL, "wan-ingress"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if deletes.Load() != 1 {
		t.Fatalf("deleted %d time(s), want 1", deletes.Load())
	}
}

func TestDeregisterToleratesAnExpiredName(t *testing.T) {
	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer reg.Close()
	if err := Deregister(context.Background(), reg.URL, "wan-ingress"); err != nil {
		t.Fatalf("a 404 is success, got %v", err)
	}
	if err := Deregister(context.Background(), "", "wan-ingress"); err != nil {
		t.Fatalf("no registry is a no-op, got %v", err)
	}
}
