package ingress

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigFileRefusesTypos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"agreements":"a.json","token":"t","bogus":true}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfigFile(path); err == nil {
		t.Fatal("an unknown key was accepted; a typo could leave the ingress defensive by accident")
	}

	if err := os.WriteFile(path, []byte(`{"agreements":"a.json","token":"t","border_key":"k"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Agreements != "a.json" || cfg.Token != "t" || cfg.BorderKey != "k" {
		t.Fatalf("config = %+v", cfg)
	}
}

// A reload adds traffic without a restart and does not reset what the monitor
// has already observed. The runtime switch is the boundary: a request sees one
// whole policy, never a half-swapped one.
func TestReloadSwapsPolicyAndKeepsCounters(t *testing.T) {
	observer := NewObserver()
	first, err := NewServer(Config{Token: testToken, BorderKey: testKey}, observer)
	if err != nil {
		t.Fatalf("first server: %v", err)
	}
	runtime := &Runtime{}
	runtime.Swap(first)
	front := httptest.NewServer(runtime)
	defer front.Close()

	status, _ := doRequest(t, front.Client(), http.MethodGet, front.URL+"/resolve?name=42", authHeaders())
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("before reload status = %d, want 422", status)
	}

	agreements := filepath.Join(t.TempDir(), "agreements.json")
	if err := os.WriteFile(agreements, []byte(testAgreement), 0o600); err != nil {
		t.Fatalf("write agreements: %v", err)
	}
	second, err := NewServer(Config{
		Agreements: agreements, Token: testToken, BorderKey: testKey,
	}, observer)
	if err != nil {
		t.Fatalf("second server: %v", err)
	}
	runtime.Swap(second)

	status, body := doRequest(t, front.Client(), http.MethodGet, front.URL+"/resolve?name=42", authHeaders())
	if status != http.StatusOK || !strings.Contains(string(body), `"decision":"forward"`) {
		t.Fatalf("after reload status = %d, body = %s", status, body)
	}
	status, body = doRequest(t, front.Client(), http.MethodGet, front.URL+"/status", nil)
	if status != http.StatusOK || !strings.Contains(string(body), `"unknown":1`) {
		t.Fatalf("counters were reset by reload: status = %d, body = %s", status, body)
	}
}

// A bad reload is rejected before the swap, so the old policy keeps serving.
func TestBadReloadKeepsTheLastGoodPolicy(t *testing.T) {
	observer := NewObserver()
	first, err := NewServer(Config{Token: testToken, BorderKey: testKey}, observer)
	if err != nil {
		t.Fatalf("first server: %v", err)
	}
	runtime := &Runtime{}
	runtime.Swap(first)
	front := httptest.NewServer(runtime)
	defer front.Close()

	bad := filepath.Join(t.TempDir(), "agreements.json")
	if err := os.WriteFile(bad, []byte(`{not json`), 0o600); err != nil {
		t.Fatalf("write bad agreements: %v", err)
	}
	if _, err := NewServer(Config{Agreements: bad, Token: testToken}, observer); err == nil {
		t.Fatal("a malformed agreement file was accepted")
	}
	// No swap happened — exactly what the SIGHUP path does on error.
	status, _ := doRequest(t, front.Client(), http.MethodGet, front.URL+"/resolve?name=42", authHeaders())
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want the last good quarantine", status)
	}
}

func TestRuntimeWithoutAPolicyIsNotReady(t *testing.T) {
	runtime := &Runtime{}
	front := httptest.NewServer(runtime)
	defer front.Close()
	status, _ := doRequest(t, front.Client(), http.MethodGet, front.URL+"/resolve?name=42", nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", status)
	}
}

// The supervisor is part of the contract: a crash must not be the end of the
// process. The plist is checked as text so the gate runs on Linux too.
func TestLaunchdServiceKeepsTheProcessAlive(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "scripts", "wan-ingress.plist"))
	if err != nil {
		t.Fatalf("read service definition: %v", err)
	}
	text := string(body)
	for _, key := range []string{"<key>RunAtLoad</key>", "<key>KeepAlive</key>", "wan-ingress"} {
		if !strings.Contains(text, key) {
			t.Fatalf("service definition is missing %s", key)
		}
	}
	for _, enabled := range []string{"<key>RunAtLoad</key>\n\t<true/>", "<key>KeepAlive</key>\n\t<true/>"} {
		if !strings.Contains(text, enabled) {
			t.Fatalf("service definition does not enable %s", enabled)
		}
	}
}
