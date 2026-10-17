package workspaces

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRegisterSpeaksToTheRegistry pins the one wire shape we depend on that is
// *not* ours: the lab's registry. The live registry is smoke-alarm
// (`POST /isotope/register` → a record with a trust rung); this is the fast guard
// that we ask it the way it answers.
func TestRegisterSpeaksToTheRegistry(t *testing.T) {
	var got isotopeRegistrationBody
	var path string

	reg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("the registry received %s, not a registration: %v", body, err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": got.Name, "role": got.Role, "endpoint": got.Endpoint,
			"protocol": got.Protocol, "trust_rung": 2, "rung_name": "Harness Tools",
		})
	}))
	defer reg.Close()

	rec, err := Register(context.Background(), reg.URL, SessionName("42"), "127.0.0.1:9999")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if path != "/isotope/register" {
		t.Errorf("posted to %q, want /isotope/register", path)
	}
	if got.Name != "workspace-42" || got.Role != "standalone" ||
		got.Endpoint != "127.0.0.1:9999" || got.Protocol != "mcp" {
		t.Errorf("registration = %+v, want the session's name/role/endpoint/protocol", got)
	}
	if rec.TrustRung != 2 || rec.RungName != "Harness Tools" {
		t.Errorf("record = %+v, want the registry's rung (the judgement is not ours)", rec)
	}
}

// A session with no registry configured does not register — and says so, rather
// than silently appearing to have.
func TestRegisterWithoutARegistryIsAnError(t *testing.T) {
	if _, err := Register(context.Background(), "", SessionName("42"), "127.0.0.1:9999"); err == nil {
		t.Fatal("expected an error with no registry configured")
	}
}

// isotopeRegistrationBody mirrors isotope.Registration; declared here so the test
// asserts the *shape on the wire*, not our own struct's field names.
type isotopeRegistrationBody struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Endpoint string `json:"endpoint"`
	Protocol string `json:"protocol"`
}
