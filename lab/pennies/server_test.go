package pennies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rpc performs one JSON-RPC call against the harness.
func rpc(t *testing.T, srv *httptest.Server, method string, params any) map[string]any {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return decoded
}

func call(t *testing.T, srv *httptest.Server, name string, args map[string]any) map[string]any {
	t.Helper()
	return rpc(t, srv, "tools/call", map[string]any{"name": name, "arguments": args})
}

func structured(t *testing.T, reply map[string]any) map[string]any {
	t.Helper()
	result, ok := reply["result"].(map[string]any)
	if !ok {
		return nil
	}
	fields, _ := result["structuredContent"].(map[string]any)
	return fields
}

func TestDynamicToolsAreInstalled(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: true}))
	defer srv.Close()

	reply := rpc(t, srv, "tools/list", map[string]any{})
	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/list: %+v", reply)
	}
	tools, _ := result["tools"].([]any)

	names := map[string]bool{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"pennies.status", "barista.manifest", "pennies.grant", "barista.path"} {
		if !names[want] {
			t.Errorf("%s was not installed; got %v", want, names)
		}
	}
	if names["pennies.bonus"] {
		t.Error("the undeclared fixture was installed without --register-undeclared")
	}
}

func TestUndeclaredToolIsOptional(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: true, Undeclared: true}))
	defer srv.Close()

	reply := call(t, srv, "pennies.bonus", map[string]any{})
	if fields := structured(t, reply); fields == nil {
		t.Fatalf("pennies.bonus was not installed: %+v", reply)
	}
}

func TestPricedToolRefusesWithoutPenny(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: true}))
	defer srv.Close()

	reply := call(t, srv, "barista.path", map[string]any{})
	errObj, ok := reply["error"].(map[string]any)
	if !ok {
		t.Fatalf("barista.path answered without a penny: %+v", reply)
	}
	if code, _ := errObj["code"].(float64); int(code) != -32001 {
		t.Errorf("refusal code = %v, want -32001 (no session here)", errObj["code"])
	}
}

func TestPricedToolAcceptsGrantedPenny(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: true}))
	defer srv.Close()

	granted := structured(t, call(t, srv, "pennies.grant", map[string]any{}))
	penny, _ := granted["penny"].(string)
	if penny == "" {
		t.Fatalf("no penny issued: %+v", granted)
	}

	fields := structured(t, call(t, srv, "barista.path", map[string]any{"penny": penny}))
	if fields == nil {
		t.Fatal("barista.path refused a penny it had just issued")
	}
	if path, _ := fields["path"].(string); !strings.HasPrefix(path, "/api/v1/turn/") {
		t.Errorf("path = %q", path)
	}
}

// The path vanishes: a second grant rotates the surface and the first penny is
// no longer a credential.
func TestStalePennyIsRefused(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: true}))
	defer srv.Close()

	first := structured(t, call(t, srv, "pennies.grant", map[string]any{}))
	stale, _ := first["penny"].(string)
	second := structured(t, call(t, srv, "pennies.grant", map[string]any{}))
	fresh, _ := second["penny"].(string)
	if stale == fresh {
		t.Fatal("two grants issued the same penny")
	}

	if reply := call(t, srv, "barista.path", map[string]any{"penny": stale}); reply["error"] == nil {
		t.Error("a penny from a previous rotation was accepted")
	}
	if reply := call(t, srv, "barista.path", map[string]any{"penny": fresh}); reply["error"] != nil {
		t.Errorf("the current penny was refused: %+v", reply)
	}
}

func TestOpenGateServesPricedTool(t *testing.T) {
	srv := httptest.NewServer(New(Options{Enforce: false}))
	defer srv.Close()

	if fields := structured(t, call(t, srv, "barista.path", map[string]any{})); fields == nil {
		t.Fatal("an unenforced surface still refused barista.path")
	}
}

func TestManifestIsReadPerCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"generation":1}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	srv := httptest.NewServer(New(Options{Enforce: true, ManifestPath: path}))
	defer srv.Close()

	first := structured(t, call(t, srv, "barista.manifest", map[string]any{}))
	if got := first["rotation_id"]; got == "" || got == nil {
		t.Errorf("handshake carried no rotation id: %+v", first)
	}

	if err := os.WriteFile(path, []byte(`{"generation":2}`), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	second := structured(t, call(t, srv, "barista.manifest", map[string]any{}))
	manifest, _ := second["manifest"].(map[string]any)
	if got, _ := manifest["generation"].(float64); got != 2 {
		t.Errorf("the handshake served a stale manifest: %+v", manifest)
	}
}
