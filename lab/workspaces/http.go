package workspaces

import (
	"io"
	"net/http"
)

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
		writeRPC(w, s.protocol.Handle(r.Context(), body))

	case http.MethodGet:
		// A human — and a plain status check — can see which session this is.
		writeRPC(w, mustJSON(map[string]any{
			"ok":              true,
			"session":         s.protocol.store.Session(),
			"protocolVersion": ProtocolVersion,
		}))

	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeRPC(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
