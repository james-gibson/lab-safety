package ingress

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func initialize(t *testing.T, client *http.Client, url string) map[string]any {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d", resp.StatusCode)
	}
	var answer struct {
		Result map[string]any `json:"result"`
		Error  *rpcError      `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&answer); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if answer.Error != nil {
		t.Fatalf("initialize error: %+v", answer.Error)
	}
	return answer.Result
}

// The monitor's view of a defensive ingress: it answers the handshake but
// advertises no forwarding. smoke-alarm reads that as degraded (adhd yellow).
func TestDefensiveIngressIsDegradedToTheMonitor(t *testing.T) {
	srv := testServer(t, NewEmptyResolver(), &Forwarder{}, "")
	defer srv.Close()

	result := initialize(t, srv.Client(), srv.URL+"/mcp")
	if result["capabilities"] == nil {
		t.Fatal("initialize result has no capabilities object")
	}
	caps := result["capabilities"].(map[string]any)
	if _, ok := caps["ingress"]; ok {
		t.Fatalf("a defensive ingress advertised forwarding: %v", caps)
	}
	info := result["serverInfo"].(map[string]any)
	if info["name"] != "wan-ingress" {
		t.Fatalf("serverInfo.name = %v, want wan-ingress", info["name"])
	}
}

// Once real agreed traffic has crossed the border, the capability is a promise
// the surface can keep: smoke-alarm reads healthy (adhd green).
func TestForwardingIngressIsHealthyToTheMonitor(t *testing.T) {
	border := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer border.Close()

	resolver, err := LoadAgreements(strings.NewReader(`{"agreements":[{"id":"42","namespace":"session-42","border":"` + border.URL + `","aliases":["42"]}]}`))
	if err != nil {
		t.Fatalf("load agreements: %v", err)
	}
	srv := testServer(t, resolver, &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, _ := doRequest(t, srv.Client(), http.MethodPost, srv.URL+"/forward?name=42", authHeaders())
	if status != http.StatusOK {
		t.Fatalf("forward status = %d, want 200", status)
	}

	result := initialize(t, srv.Client(), srv.URL+"/mcp")
	caps := result["capabilities"].(map[string]any)
	if caps["ingress"] != "forward" {
		t.Fatalf("capabilities = %v, want ingress=forward", caps)
	}
}

func TestMonitorStatusReportsDefensiveState(t *testing.T) {
	srv := testServer(t, NewEmptyResolver(), &Forwarder{}, "")
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"ingress/status","params":{}}`
	resp, err := srv.Client().Post(srv.URL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var answer struct {
		Result probeStatus `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !answer.Result.Defensive || answer.Result.Ready {
		t.Fatalf("status = %+v, want defensive and not ready", answer.Result)
	}
}

func TestMonitorRejectsUnknownMethods(t *testing.T) {
	srv := testServer(t, NewEmptyResolver(), &Forwarder{}, "")
	defer srv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`
	resp, err := srv.Client().Post(srv.URL+"/mcp", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var answer struct {
		Error *rpcError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if answer.Error == nil || answer.Error.Code != -32601 {
		t.Fatalf("error = %+v, want method not found", answer.Error)
	}
}
