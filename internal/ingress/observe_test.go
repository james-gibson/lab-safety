package ingress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Registration is by name, in the same shape lab/workspaces uses. The rung is
// the registry's judgement, so this binary never claims one for itself.
func TestRegisterByName(t *testing.T) {
	type registration struct {
		Name     string `json:"name"`
		Role     string `json:"role"`
		Endpoint string `json:"endpoint"`
		Protocol string `json:"protocol"`
	}
	var got registration
	var path, method string
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode registration: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"wan-ingress","trust_rung":2}`))
	}))
	defer registry.Close()

	status, body, err := Register(context.Background(), registry.URL, "wan-ingress", "http://127.0.0.1:8443")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if status != http.StatusCreated || string(body) != `{"name":"wan-ingress","trust_rung":2}` {
		t.Fatalf("registry replied %d %s", status, body)
	}
	if method != http.MethodPost || path != "/isotope/register" {
		t.Fatalf("registry saw %s %s, want POST /isotope/register", method, path)
	}
	if got.Name != "wan-ingress" || got.Role != "standalone" || got.Protocol != "http" || got.Endpoint != "http://127.0.0.1:8443" {
		t.Fatalf("registration = %+v", got)
	}
}

func TestObserverCountsByReason(t *testing.T) {
	observer := NewObserver()
	observer.Record(Decision{Decision: "forward"})
	observer.Record(quarantine("unknown"))
	observer.Record(quarantine("unknown"))
	observer.Record(quarantine("ambiguous"))

	recorder := httptest.NewRecorder()
	observer.ServeStatus(true, recorder, httptest.NewRequest(http.MethodGet, "/status", nil))
	var counts statusBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &counts); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if counts.Forwarded != 1 || counts.Quarantined["unknown"] != 2 || counts.Quarantined["ambiguous"] != 1 {
		t.Fatalf("status = %+v", counts)
	}
}
