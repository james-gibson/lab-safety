package ingress

import (
	"encoding/json"
	"io"
	"net/http"
)

// This is a probe surface, not a general MCP server. smoke-alarm's MCP prober
// sends a JSON-RPC handshake and compares the advertised capabilities with the
// target's declared min_capabilities. A defensive ingress answers but advertises
// no forwarding, so the alarm reads degraded (adhd: yellow); a forwarding
// ingress advertises "ingress": "forward" and reads healthy (adhd: green).
// If the process is down the probe fails outright and reads unhealthy (red).
const (
	probeProtocolVersion = "2025-06-18"
	probeName            = "wan-ingress"
	probeVersion         = "1"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type probeStatus struct {
	Ready       bool              `json:"ready"`
	Defensive   bool              `json:"defensive"`
	Forwarded   uint64            `json:"forwarded"`
	Quarantined map[string]uint64 `json:"quarantined"`
}

func (s *Server) serveMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil || len(body) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil || req.JSONRPC != "2.0" || req.Method == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// A notification has no id and no response; the alarm only checks the status.
	if req.Method == "notifications/initialized" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = s.initializeResult()
	case "ingress/status":
		forwarded, quarantined := s.Observer.Snapshot()
		ready := s.ready()
		resp.Result = probeStatus{
			Ready: ready, Defensive: !ready || forwarded == 0,
			Forwarded: forwarded, Quarantined: quarantined,
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) initializeResult() map[string]any {
	capabilities := map[string]any{}
	if s.ready() && s.Observer.Forwarded() > 0 {
		capabilities["ingress"] = "forward"
	}
	return map[string]any{
		"protocolVersion": probeProtocolVersion,
		"capabilities":    capabilities,
		"serverInfo": map[string]any{
			"name":    probeName,
			"version": probeVersion,
		},
	}
}
