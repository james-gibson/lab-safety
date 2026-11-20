package ingress

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Server is the WAN front: thin admission, resolution by agreement, signed
// forwarding, and observable refusals. It is built from the feature in
// sheets/ingress/wan-ingress.feature.
type Server struct {
	Resolver  *Resolver
	Forwarder *Forwarder
	Token     string
	Observer  *Observer
}

// Handler wires the five surfaces. Defaults are fail-closed: no agreements
// means every name is unknown; no token means every caller is untrusted.
func (s *Server) Handler() http.Handler {
	if s.Observer == nil {
		s.Observer = NewObserver()
	}
	if s.Resolver == nil {
		s.Resolver = NewEmptyResolver()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", ServeHealthz)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ServeReadyz(s.ready(), w, r)
	})
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		s.Observer.ServeStatus(s.ready(), w, r)
	})
	mux.HandleFunc("GET /resolve", s.serveResolve)
	mux.HandleFunc("POST /forward", s.serveForward)
	return mux
}

func (s *Server) ready() bool {
	return s != nil && s.Resolver != nil && s.Token != "" && s.Forwarder.Ready()
}

func (s *Server) serveResolve(w http.ResponseWriter, r *http.Request) {
	if !Admit(r.Header.Get("Authorization"), s.Token) {
		refusal := quarantine("untrusted")
		s.Observer.Record(refusal)
		writeDecision(w, http.StatusForbidden, refusal)
		return
	}

	decision := s.Resolver.Resolve(rawQueryValue(r.URL.RawQuery, "name"))
	s.Observer.Record(decision)
	status := http.StatusUnprocessableEntity
	if decision.Decision == "forward" {
		status = http.StatusOK
	}
	writeDecision(w, status, decision)
}

func (s *Server) serveForward(w http.ResponseWriter, r *http.Request) {
	if !Admit(r.Header.Get("Authorization"), s.Token) {
		refusal := quarantine("untrusted")
		s.Observer.Record(refusal)
		writeDecision(w, http.StatusForbidden, refusal)
		return
	}

	rawName := rawQueryValue(r.URL.RawQuery, "name")
	decision := s.Resolver.Resolve(rawName)
	if decision.Decision != "forward" {
		s.Observer.Record(decision)
		writeDecision(w, http.StatusUnprocessableEntity, decision)
		return
	}
	if !s.Forwarder.Ready() {
		refusal := quarantine("border_unconfigured")
		s.Observer.Record(refusal)
		writeDecision(w, http.StatusUnprocessableEntity, refusal)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxForwardBody+1))
	if err != nil || len(body) > maxForwardBody {
		refusal := quarantine("malformed")
		s.Observer.Record(refusal)
		writeDecision(w, http.StatusUnprocessableEntity, refusal)
		return
	}

	status, err := s.Forwarder.Forward(r.Context(), *decision.Route, rawName, body)
	if err != nil {
		refusal := quarantine("upstream")
		s.Observer.Record(refusal)
		writeDecision(w, http.StatusBadGateway, refusal)
		return
	}

	forwarded := Decision{Decision: "forward", Route: decision.Route, UpstreamStatus: status}
	s.Observer.Record(forwarded)
	writeDecision(w, http.StatusOK, forwarded)
}

func writeDecision(w http.ResponseWriter, status int, decision Decision) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(decision)
}

// rawQueryValue takes the raw (still percent-encoded) value so ParseName is
// the single decoder. url.Values.Get would decode once here and ParseName would
// decode twice — a classic way to smuggle a second interpretation.
func rawQueryValue(rawQuery, key string) string {
	for _, pair := range strings.Split(rawQuery, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}
