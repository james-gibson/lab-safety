// Package pennies is a harness surface for testing lab-safety's invariants.
//
// It serves a streamable-HTTP MCP endpoint whose tools are installed at
// runtime, the way adhd's per-project proxies are installed by
// adhd.cluster.join:
//
//	barista.manifest   free — the handshake: returns the policy manifest and
//	                   the current rotation id
//	pennies.grant      free — issues a penny (a token), bound to the rotation it
//	                   was issued in
//	barista.path       priced, gated — the vanishing path; refuses without a
//	                   penny, and refuses a penny from an earlier rotation
//
// The two flags exist so the harness can be *wrong* on purpose, which is what a
// checker is for:
//
//	--enforce=false      serve the priced tool without a penny (GATE-OPEN)
//	--register-undeclared
//	                     install a tool no sheet declares (PROBE-TOOLS-EXTRA)
//
// Policy is declared in sheets/pennies.md; the price and the enforced gate are
// declared in policy/skills.yaml.
package pennies

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// Options configures the harness surface.
type Options struct {
	// Enforce makes the priced tool require a valid penny. When false the tool
	// answers anyway: the gate is open, and the checker must say so.
	Enforce bool
	// Undeclared installs a tool that no safety data sheet declares.
	Undeclared bool
	// ManifestPath is read on every call, so a regenerated manifest is served
	// without a restart. An empty path serves an empty object.
	ManifestPath string
}

// Tool is one installed tool.
type Tool struct {
	Name        string
	Description string
	// Priced tools require a penny and are the ones a sheet's §3 marks as the
	// surface's paid path.
	Priced bool
	// Free-form JSON schema, kept minimal: the harness is not testing schema
	// canonicalisation.
	InputSchema map[string]any
}

// surface is the harness state: the rotation, the pennies issued in it, and
// whether the gate is closed.
type surface struct {
	opts Options

	mu       sync.Mutex
	tools    []Tool
	rotation string
	pennies  map[string]string // penny -> rotation it was issued in
	lastPen  string
}

// New builds the harness surface and installs its tools. Installation happens
// here, at runtime, rather than being a fixed list compiled into a caller: that
// is what "dynamic" means in the sheets (adhd.cluster.join installs tools the
// same way).
func New(opts Options) *surface {
	s := &surface{opts: opts, pennies: map[string]string{}}
	s.rotate()
	s.install()
	return s
}

// install is the runtime registration step.
func (s *surface) install() {
	s.tools = []Tool{
		{
			Name:        "pennies.status",
			Description: "Rotation id, price and gate state of the pennies harness.",
		},
		{
			Name:        "barista.manifest",
			Description: "The barista handshake: the policy manifest and the current rotation id.",
		},
		{
			Name:        "pennies.grant",
			Description: "Issue a penny bound to the current rotation.",
		},
		{
			Name:        "barista.path",
			Description: "The vanishing path. Requires a penny from the current rotation.",
			Priced:      true,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"penny": map[string]any{"type": "string"},
				},
			},
		},
	}
	if s.opts.Undeclared {
		s.tools = append(s.tools, Tool{
			Name:        "pennies.bonus",
			Description: "Not declared by any sheet: the harness's PROBE-TOOLS-EXTRA fixture.",
		})
	}
}

// ServeHTTP implements the streamable-HTTP transport: one JSON-RPC request per
// POST, answered with a JSON body.
func (s *surface) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, nil, -32700, "parse error")
		return
	}

	// A notification has no id and gets no response body.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	switch req.Method {
	case "initialize":
		writeRPC(w, req.ID, map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "pennies", "version": "0.1.0"},
		})
	case "ping":
		writeRPC(w, req.ID, map[string]any{})
	case "tools/list":
		writeRPC(w, req.ID, map[string]any{"tools": s.toolList()})
	case "tools/call":
		s.callTool(w, req.ID, req.Params)
	default:
		// -32601 is the spec's "method not found", which is the code the lab's
		// surfaces use for a protocol mismatch (sheets/ocd-smoke-alarm.md §4).
		writeRPCError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *surface) toolList() []map[string]any {
	out := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	return out
}

func (s *surface) callTool(w http.ResponseWriter, id json.RawMessage, params json.RawMessage) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		writeRPCError(w, id, -32602, "invalid params")
		return
	}

	tool, ok := s.tool(call.Name)
	if !ok {
		writeRPCError(w, id, -32602, "unknown tool: "+call.Name)
		return
	}

	var args map[string]any
	if len(call.Arguments) > 0 {
		_ = json.Unmarshal(call.Arguments, &args)
	}

	result, err := s.invoke(tool, args)
	if err != nil {
		if _, isRefusal := err.(*refusal); isRefusal {
			writeRPCError(w, id, -32001, err.Error())
			return
		}
		writeRPCError(w, id, -32603, err.Error())
		return
	}
	writeRPC(w, id, map[string]any{
		"content": []map[string]any{{"type": "text", "text": mustJSON(result)}},
		// structuredContent lets a caller read the penny without re-parsing
		// the text block; the checker prefers it and falls back to text.
		"structuredContent": result,
		"isError":           false,
	})
}

// refusal is a call the surface declines: the lab's "no session here" code,
// -32001.
type refusal struct{ reason string }

func (r *refusal) Error() string { return r.reason }

func (s *surface) invoke(tool Tool, args map[string]any) (map[string]any, error) {
	switch tool.Name {
	case "pennies.status":
		s.mu.Lock()
		defer s.mu.Unlock()
		return map[string]any{
			"rotation_id": s.rotation,
			"price":       map[string]any{"amount": 0.01, "currency": "USD", "unit": "token"},
			"enforced":    s.opts.Enforce,
			"tools":       len(s.tools),
		}, nil

	case "barista.manifest":
		s.mu.Lock()
		rotation := s.rotation
		s.mu.Unlock()
		return map[string]any{
			"manifest":    s.manifest(),
			"rotation_id": rotation,
			"gate": map[string]any{
				"tool":     "barista.path",
				"requires": "penny",
			},
		}, nil

	case "pennies.grant":
		penny := s.grant()
		s.mu.Lock()
		rotation := s.rotation
		s.mu.Unlock()
		return map[string]any{
			"penny":       penny,
			"rotation_id": rotation,
			"price":       map[string]any{"amount": 0.01, "currency": "USD", "unit": "token"},
		}, nil

	case "barista.path":
		if err := s.admit(args); err != nil {
			return nil, err
		}
		s.mu.Lock()
		rotation := s.rotation
		s.mu.Unlock()
		return map[string]any{
			"path":        "/api/v1/turn/" + rotation,
			"rotation_id": rotation,
		}, nil

	case "pennies.bonus":
		return map[string]any{"bonus": "undocumented"}, nil
	}
	return nil, fmt.Errorf("unknown tool: %s", tool.Name)
}

// admit enforces the priced tool's gate. A penny is valid only for the rotation
// it was issued in: that is the vanishing path, and it is the part of the
// prototype that is actually enforceable server-side.
func (s *surface) admit(args map[string]any) error {
	if !s.opts.Enforce {
		return nil
	}
	penny, _ := args["penny"].(string)
	if penny == "" {
		return &refusal{reason: "no penny: barista.path requires a token"}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rotation, ok := s.pennies[penny]
	if !ok {
		return &refusal{reason: "unknown penny"}
	}
	if rotation != s.rotation {
		return &refusal{reason: "stale penny: issued in a previous rotation"}
	}
	return nil
}

// grant rotates the surface and issues a penny in the new rotation. The
// rotation is what makes the path vanish: every grant invalidates the pennies
// of the previous rotation.
func (s *surface) grant() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotate()
	penny := "pnn_" + randomHex(8)
	s.pennies = map[string]string{penny: s.rotation}
	s.lastPen = penny
	return penny
}

func (s *surface) rotate() {
	s.rotation = "rot_" + randomHex(6)
}

// manifest reads the policy manifest on every call, so a regenerated object is
// served without a restart.
func (s *surface) manifest() any {
	if s.opts.ManifestPath == "" {
		return map[string]any{}
	}
	raw, err := os.ReadFile(s.opts.ManifestPath) //nolint:gosec // G304: the harness is pointed at the repo's own manifest
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}

func (s *surface) tool(name string) (Tool, bool) {
	for _, t := range s.tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Rotation exposes the current rotation for tests.
func (s *surface) Rotation() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotation
}

// LastPenny exposes the most recently issued penny for tests.
func (s *surface) LastPenny() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPen
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", 0)
	}
	return hex.EncodeToString(b)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func writeRPC(w http.ResponseWriter, id json.RawMessage, result any) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		writeRPCError(w, id, -32603, "encode result")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}
