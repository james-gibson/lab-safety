package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInviteProxyResolvesByNameBeforeForwarding(t *testing.T) {
	var gotAuth, gotPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"capabilities":{}}}`))
	}))
	defer backend.Close()

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/isotope/list" {
			t.Errorf("registry path = %s, want /isotope/list", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]isotopeRecord{{
			Name:         "wan-ingress",
			Endpoint:     backend.URL,
			RegisteredAt: time.Now().UTC(),
		}})
	}))
	defer registry.Close()

	proxy := httptest.NewServer(newHandler(registry.URL, "wan-ingress", "invite-token", registry.Client()))
	defer proxy.Close()

	resp, err := proxy.Client().Post(proxy.URL+"/mcp", "application/json", http.NoBody)
	if err != nil {
		t.Fatalf("post via proxy: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if gotAuth != "Bearer invite-token" {
		t.Errorf("Authorization = %q, want the invite's grant", gotAuth)
	}
	if gotPath != "/mcp" {
		t.Errorf("path = %q, want /mcp", gotPath)
	}
}

func TestInviteProxyHonoursXInviteAndCallerAuthorization(t *testing.T) {
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]isotopeRecord{
			{Name: "wan-ingress", Endpoint: "http://127.0.0.1:1", RegisteredAt: time.Now().UTC()},
			{Name: "peer-egress", Endpoint: backend.URL, RegisteredAt: time.Now().Add(time.Second).UTC()},
		})
	}))
	defer registry.Close()

	proxy := httptest.NewServer(newHandler(registry.URL, "wan-ingress", "invite-token", registry.Client()))
	defer proxy.Close()

	req, err := http.NewRequest(http.MethodPost, proxy.URL+"/forward?name=42", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Invite", "peer-egress")
	req.Header.Set("Authorization", "Bearer red-team-token")
	resp, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatalf("do via proxy: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if gotAuth != "Bearer red-team-token" {
		t.Errorf("Authorization = %q, want the caller's own credential to survive", gotAuth)
	}
}

func TestInviteProxyFailsClosedWhenInviteIsUnknown(t *testing.T) {
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]isotopeRecord{})
	}))
	defer registry.Close()

	proxy := httptest.NewServer(newHandler(registry.URL, "wan-ingress", "invite-token", registry.Client()))
	defer proxy.Close()

	resp, err := proxy.Client().Get(proxy.URL + "/mcp")
	if err != nil {
		t.Fatalf("get via proxy: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 for an unresolvable invite", resp.StatusCode)
	}
}
