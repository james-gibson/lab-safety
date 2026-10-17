package workspaces

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// handshake speaks exactly what ocd-smoke-alarm's MCP prober speaks: one
// JSON-RPC 2.0 POST per required method, to the endpoint, expecting 2xx and a
// JSON-RPC body whose `error` is absent (smoke-alarm/internal/engine →
// `httpHandshakeMethods`, `httpHandshakeParams`, `probeJSONRPCHandshakeHTTP`).
//
// The point of replaying *its* request shape is that the contract is not ours:
// if this passes and the live alarm still goes red, the defect is in the surface,
// not in the guess about what the prober sends.
func handshake(t *testing.T, endpoint, method string) map[string]any {
	t.Helper()
	params := map[string]any{}
	if method == "initialize" {
		params = map[string]any{
			"protocolVersion": ProtocolVersion,
			"clientInfo":      map[string]any{"name": "ocd-smoke-alarm", "version": "0.1.0"},
			"capabilities":    map[string]any{},
		}
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s: status %d — the prober reads any non-2xx as unhealthy", method, resp.StatusCode)
	}
	var rpc struct {
		Result map[string]any `json:"result"`
		Error  *rpcError      `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatalf("%s: invalid JSON-RPC body: %v", method, err)
	}
	if rpc.Error != nil {
		t.Fatalf("%s: the surface refused: %s", method, rpc.Error.Message)
	}
	return rpc.Result
}

// liveServer seeds a store from the gate and serves its surface on a real
// socket, so the test crosses the same boundary the lab does.
func liveServer(t *testing.T) (endpoint string, spec SpecRef) {
	t.Helper()
	spec = gateRef(t)
	drv, err := NewTmpDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = drv.Close() })

	s := New(drv, "42")
	if err := Seed(context.Background(), s, spec); err != nil {
		t.Fatalf("seed: %v", err)
	}
	srv := NewServer(s)
	if err := BindTools(srv, spec); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts.URL + "/mcp", spec
}

// names reads the name-bearing field out of a JSON-RPC list result.
func names(res map[string]any, key, field string) []string {
	raw, _ := res[key].([]any)
	out := []string{}
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			if f, ok := m[field].(string); ok {
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestTheWireContractIsTheProbers is A8 at rung 0: the handshake a separate
// process performs — over a socket, not through the driver — is answered.
func TestTheWireContractIsTheProbers(t *testing.T) {
	endpoint, _ := liveServer(t)

	// the strict profile's methods, in the prober's order
	for _, method := range []string{"initialize", "tools/list", "resources/list"} {
		handshake(t, endpoint, method)
	}

	init := handshake(t, endpoint, "initialize")
	if init["protocolVersion"] != ProtocolVersion {
		t.Errorf("initialize protocolVersion = %v, want %s", init["protocolVersion"], ProtocolVersion)
	}
	info, _ := init["serverInfo"].(map[string]any)
	if info["name"] != "workspace-42" {
		t.Errorf("initialize serverInfo = %v, want workspace-42", init["serverInfo"])
	}
}

// TestTheSurfaceServesTheSpec is the spec check, taken on the wire: what
// `tools/list`, `resources/list` and `prompts/list` answer is what the spec
// declares. The seed is the spec inverted; the wire is the measurement.
func TestTheSurfaceServesTheSpec(t *testing.T) {
	endpoint, spec := liveServer(t)
	want, err := spec.Expectation()
	if err != nil {
		t.Fatal(err)
	}

	if got := names(handshake(t, endpoint, "tools/list"), "tools", "name"); !equalStrings(got, want.Tools) {
		t.Errorf("tools/list = %v, want the spec's %v", got, want.Tools)
	}
	if got := names(handshake(t, endpoint, "resources/list"), "resources", "uri"); !equalStrings(got, want.Resources) {
		t.Errorf("resources/list = %v, want the spec's %v", got, want.Resources)
	}
	if got := names(handshake(t, endpoint, "prompts/list"), "prompts", "name"); !equalStrings(got, want.Prompts) {
		t.Errorf("prompts/list = %v, want the spec's %v", got, want.Prompts)
	}
}

// TestToolCallCrossesTheWire: `tools/call` is a file write plus a result read
// (§2), and it works from the other side of a socket.
func TestToolCallCrossesTheWire(t *testing.T) {
	endpoint, _ := liveServer(t)

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 7, "method": "tools/call",
		"params": map[string]any{"name": "echo", "arguments": "hi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	var rpc struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error != nil {
		t.Fatalf("tools/call refused: %s", rpc.Error.Message)
	}
	// the stand-in hands back what it was given; `arguments` arrives as raw JSON
	if len(rpc.Result.Content) != 1 || rpc.Result.Content[0].Text != `"hi"` {
		t.Errorf("tools/call content = %+v, want the echoed argument", rpc.Result.Content)
	}
}

// TestARefusedMethodIsStillHTTP200 keeps the two failure shapes distinct: the
// prober must be able to tell "answered and refused" (a JSON-RPC error) from
// "did not answer" (non-2xx).
func TestARefusedMethodIsStillHTTP200(t *testing.T) {
	endpoint, _ := liveServer(t)

	resp, err := http.Post(endpoint, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"no/such/method"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the refusal belongs in the body", resp.StatusCode)
	}
	var rpc struct {
		Error *rpcError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error == nil || rpc.Error.Code != -32601 {
		t.Errorf("error = %+v, want -32601", rpc.Error)
	}
}

// TestGetReportsTheSession: a human, and a plain status check, can ask the
// surface which session it is.
func TestGetReportsTheSession(t *testing.T) {
	endpoint, _ := liveServer(t)

	resp, err := http.Get(strings.TrimSuffix(endpoint, "/mcp"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["session"] != "42" || got["protocolVersion"] != ProtocolVersion {
		t.Errorf("GET = %v, want the session and the protocol version", got)
	}
}

// TestASurfaceWithNoSessionAnswersNothing: plane `42` — *not present* — is
// reachable on the wire. Delete the session's own mark and the surface, still
// running, refuses: the light tracks the session, not the process.
func TestASurfaceWithNoSessionAnswersNothing(t *testing.T) {
	ctx := context.Background()
	spec := gateRef(t)
	drv, err := NewTmpDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = drv.Close() })

	s := New(drv, "42")
	if err := Seed(ctx, s, spec); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewServer(s))
	t.Cleanup(ts.Close)
	endpoint := ts.URL + "/mcp"

	// there is a session: the surface answers
	handshake(t, endpoint, "initialize")

	// the session goes away; the process does not
	if err := drv.Delete(ctx, PathTask); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Post(endpoint, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the refusal belongs in the body", resp.StatusCode)
	}
	var rpc struct {
		ID    int       `json:"id"`
		Error *rpcError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error == nil || rpc.Error.Code != -32001 {
		t.Fatalf("error = %+v, want -32001 (no session here)", rpc.Error)
	}
	if rpc.ID != 4 {
		t.Errorf("id = %d, want the request's 4", rpc.ID)
	}
}

// capabilityFamilies reads the advertised capability families out of an
// `initialize` result.
func capabilityFamilies(res map[string]any) []string {
	raw, _ := res["capabilities"].(map[string]any)
	out := []string{}
	for name := range raw {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestTheSurfaceAdvertisesOnlyWhatItCanServe: a capability is a promise. Lose
// part of the session and the surface stops promising it — which is what makes the
// middle rung readable by a target that declared it (register §2, smoke-alarm#34).
func TestTheSurfaceAdvertisesOnlyWhatItCanServe(t *testing.T) {
	ctx := context.Background()
	spec := gateRef(t)
	drv, err := NewTmpDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = drv.Close() })

	s := New(drv, "42")
	if err := Seed(ctx, s, spec); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewServer(s))
	t.Cleanup(ts.Close)
	endpoint := ts.URL + "/mcp"

	// the whole session: every family the spec declares is promised
	if got := capabilityFamilies(handshake(t, endpoint, "initialize")); !equalStrings(got, []string{"prompts", "resources", "tools"}) {
		t.Errorf("advertised = %v, want all three families", got)
	}

	// the session loses what the spec declares
	if err := drv.Delete(ctx, DirTools+"/echo@"); err != nil {
		t.Fatal(err)
	}

	got := capabilityFamilies(handshake(t, endpoint, "initialize"))
	if contains(got, "tools") {
		t.Errorf("advertised = %v, want no tools once the session lost them", got)
	}
	// it still answers — the session exists, it is just not what was declared
	handshake(t, endpoint, "tools/list")
}

// TestSeedIsTheSpecInverted: every file the spec names exists in the store, and
// the store's own surfaces present the names the spec declares — on both hosts.
// The spec's layout and the store's conventions must be one layout.
func TestSeedIsTheSpecInverted(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		spec := gateRef(t)
		s := New(d, "42")

		if err := Seed(ctx, s, spec); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if err := CheckLayout(ctx, s, spec); err != nil {
			t.Errorf("the spec's layout and the store's disagree: %v", err)
		}

		want, err := spec.Expectation()
		if err != nil {
			t.Fatal(err)
		}
		tools, err := s.ToolsList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(tools, want.Tools) {
			t.Errorf("tools = %v, want %v", tools, want.Tools)
		}
		res, err := s.ResourcesList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(res, want.Resources) {
			t.Errorf("resources = %v, want %v", res, want.Resources)
		}
		prompts, err := s.PromptList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !equalStrings(prompts, want.Prompts) {
			t.Errorf("prompts = %v, want %v", prompts, want.Prompts)
		}
	})
}
