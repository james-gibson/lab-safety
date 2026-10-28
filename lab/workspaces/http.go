package workspaces

import (
	"context"
	"io"
	"net/http"
	"strings"
)

// peerKey carries the caller's address from the transport into the protocol.
// The store's own record of a honeypot contact should say who knocked when the
// surface was reached over HTTP; an in-process caller has no peer and is
// recorded as such.
type peerKey struct{}

func withPeer(ctx context.Context, peer string) context.Context {
	if peer == "" {
		return ctx
	}
	return context.WithValue(ctx, peerKey{}, peer)
}

func peerFrom(ctx context.Context) string {
	p, _ := ctx.Value(peerKey{}).(string)
	return p
}

// Server is a store's MCP surface served over HTTP, so the surface can be read
// from a *different process* than the writer (register §1, rung 2).
//
// The wire contract is not ours to choose: it is the one `ocd-smoke-alarm`'s MCP
// prober speaks. Each handshake method is POSTed as a JSON-RPC 2.0 request to the
// endpoint, and each reply must be 2xx with a JSON-RPC body whose `error` is
// absent (`smoke-alarm/internal/engine`, `httpHandshakeMethods`). A JSON-RPC
// *error* is still HTTP 200 — that is what MCP does, and it is what lets the
// prober tell "answered and refused" from "did not answer".
type Server struct {
	protocol *Protocol
}

// NewServer returns a store's MCP surface, ready to serve.
func NewServer(s *Store) *Server { return &Server{protocol: NewProtocol(s)} }

// RegisterTool binds a tool name to its stand-in implementation (the "@ file").
func (s *Server) RegisterTool(name string, fn func([]byte) []byte) {
	s.protocol.RegisterTool(name, fn)
}

// Store is the session whose surface this is.
func (s *Server) Store() *Store { return s.protocol.store }

// maxBody bounds one request. A handshake is small; a runaway body is not a
// reason for the surface to fall over.
const maxBody = 1 << 20

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// One listener, two faces, distinguished by path: MCP on POST, and the
	// family's health path on GET. A second port would be one more thing to
	// allocate and publish on a host already running adhd + smoke-alarm + tuner.
	if r.Method == http.MethodGet && strings.TrimRight(r.URL.Path, "/") == "/healthz" {
		s.healthz(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			writeRPC(w, mustJSON(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			}))
			return
		}
		writeRPC(w, s.protocol.Handle(withPeer(r.Context(), r.RemoteAddr), body))

	case http.MethodGet:
		// A human — and a plain status check — can see which session this is.
		// This is identity, not health: it answers either way, and says nothing
		// about whether the session exists (for that, /healthz).
		writeRPC(w, mustJSON(map[string]any{
			"session":         s.protocol.store.Session(),
			"protocolVersion": ProtocolVersion,
		}))

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// healthz answers the family's health path — and answers about the *session*, not
// the process. adhd probes `<endpoint>/healthz` on the instruments it watches,
// so a surface co-hosted with them must not answer 200 for a live process whose
// session is gone: that is the same false signal `-32001` exists to prevent,
// told to a supervisor instead of a client. 503 means "the process is here and
// the session is not".
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	present, err := s.protocol.store.Present(r.Context())
	body := map[string]any{
		"ok":      present,
		"session": s.protocol.store.Session(),
	}
	code := http.StatusOK
	switch {
	case err != nil:
		body["ok"] = false
		body["reason"] = err.Error()
		code = http.StatusServiceUnavailable
	case !present:
		body["reason"] = "no session here"
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(mustJSON(body))
}

func writeRPC(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
