package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ProtocolVersion is the MCP protocol version this surface speaks.
const ProtocolVersion = "2024-11-05"

// Protocol is the MCP surface over a store: resources, tools and prompts, all
// backed by files, spoken as JSON-RPC 2.0 (register, minimal.feature §2).
//
// It exists so the store can be read from a *different position* than the write
// path — a reader speaks the protocol, not the driver (register §5, Phase 2).
type Protocol struct {
	store *Store
	tools map[string]func([]byte) []byte

	// mu serialises the honeypot's contact ledger: the append is a
	// read-modify-write, and two knocks landing at once must not lose one.
	mu sync.Mutex
}

// NewProtocol returns an MCP surface over a store.
func NewProtocol(s *Store) *Protocol {
	return &Protocol{store: s, tools: map[string]func([]byte) []byte{}}
}

// RegisterTool binds a tool name to its stand-in implementation (the "@ file").
func (p *Protocol) RegisterTool(name string, fn func([]byte) []byte) { p.tools[name] = fn }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Handle dispatches one JSON-RPC request and returns the response bytes.
func (p *Protocol) Handle(ctx context.Context, raw []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return mustJSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
	}
	// The surface reports its session. With no session behind it there is nothing
	// to answer, and saying so is how "not present" reaches a caller over the
	// wire — rather than a live process standing in for an existing session.
	if ok, err := p.store.Present(ctx); err == nil && !ok {
		return mustJSON(rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32001, Message: "no session here: " + p.store.Session()},
		})
	}
	res, rerr := p.dispatch(ctx, req)
	if rerr != nil {
		return mustJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: rerr})
	}
	return mustJSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
}

func (p *Protocol) dispatch(ctx context.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		// Advertise only what this session can actually serve. A capability is a
		// promise, and a surface must not make a promise its session cannot keep —
		// so a session that has lost part of itself says so here, and a target that
		// declares the missing family reads it as *"answering, but not as
		// declared"* (register §2) rather than as if nothing were missing.
		caps := map[string]any{}
		if present, err := p.availableCapabilities(ctx); err == nil {
			for _, name := range present {
				caps[name] = map[string]any{}
			}
		}
		// The resources family is present, but its delivery half is not: the
		// surface returns bytes and pushes nothing, so `subscribe` is stated false
		// rather than omitted. A caller that subscribes anyway has gone past the
		// promise and is the decoy's caller (honeypot.go).
		if _, ok := caps["resources"]; ok {
			caps["resources"] = map[string]any{"subscribe": false, "listChanged": false}
		}
		return map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    caps,
			"serverInfo": map[string]any{
				"name":    "workspace-" + p.store.Session(),
				"version": "0",
			},
		}, nil

	case "tools/list":
		names, err := p.store.ToolsList(ctx)
		if err != nil {
			return nil, internal(err)
		}
		tools := make([]map[string]any, 0, len(names))
		for _, n := range names {
			tools = append(tools, map[string]any{
				"name":        n,
				"description": "",
				"inputSchema": map[string]any{"type": "object"},
			})
		}
		return map[string]any{"tools": tools}, nil

	case "tools/call":
		var prm struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &prm); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		fn, ok := p.tools[prm.Name]
		if !ok {
			return nil, &rpcError{Code: -32601, Message: "unknown tool: " + prm.Name}
		}
		out, err := p.store.CallTool(ctx, prm.Name, prm.Arguments, fn)
		if err != nil {
			return nil, internal(err)
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": string(out)}},
		}, nil

	case "resources/list":
		uris, err := p.store.ResourcesList(ctx)
		if err != nil {
			return nil, internal(err)
		}
		rs := make([]map[string]any, 0, len(uris))
		for _, u := range uris {
			rs = append(rs, map[string]any{"uri": u, "name": u})
		}
		return map[string]any{"resources": rs}, nil

	case "resources/read":
		var prm struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &prm); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		b, err := p.store.ResourcesRead(ctx, prm.URI)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: err.Error()}
		}
		return map[string]any{
			"contents": []map[string]any{{"uri": prm.URI, "text": string(b)}},
		}, nil

	// The delivery half of `resources` is a decoy: the method answers as a real
	// surface would and records the knock, but nothing is pushed (honeypot.go,
	// sheet §11). It is here rather than in a flag so the answer and the record
	// cannot drift apart.
	case "resources/subscribe", "resources/unsubscribe":
		return p.subscribe(ctx, req.Method, req.Params)

	case "prompts/list":
		names, err := p.store.PromptList(ctx)
		if err != nil {
			return nil, internal(err)
		}
		ps := make([]map[string]any, 0, len(names))
		for _, n := range names {
			ps = append(ps, map[string]any{"name": n})
		}
		return map[string]any{"prompts": ps}, nil

	case "prompts/get":
		var prm struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &prm); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		text, err := p.store.PromptGet(ctx, prm.Name, prm.Arguments)
		if err != nil {
			return nil, &rpcError{Code: -32602, Message: err.Error()}
		}
		return map[string]any{
			"messages": []map[string]any{{"role": "user", "content": map[string]any{"type": "text", "text": text}}},
		}, nil

	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found: " + req.Method}
	}
}

func internal(err error) *rpcError { return &rpcError{Code: -32603, Message: err.Error()} }

// availableCapabilities is the promise this surface can make: a capability family
// is advertised only while the session can serve it. A family with nothing behind
// it is not advertised — that is the difference between *exists* and *works*, and
// making it visible on the wire is what lets a target check read the middle rung.
func (p *Protocol) availableCapabilities(ctx context.Context) ([]string, error) {
	var out []string

	tools, err := p.store.ToolsList(ctx)
	if err != nil {
		return nil, err
	}
	if len(tools) > 0 {
		out = append(out, "tools")
	}

	resources, err := p.store.ResourcesList(ctx)
	if err != nil {
		return nil, err
	}
	if len(resources) > 0 {
		out = append(out, "resources")
	}

	prompts, err := p.store.PromptList(ctx)
	if err != nil {
		return nil, err
	}
	if len(prompts) > 0 {
		out = append(out, "prompts")
	}

	return out, nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"jsonrpc":"2.0","error":{"code":-32603,"message":%q}}`, err.Error()))
	}
	return b
}
