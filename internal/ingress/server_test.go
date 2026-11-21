package ingress

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type capture struct {
	mu   sync.Mutex
	seen []captured
}

type captured struct {
	host, path, method, signature, namespace string
}

func (c *capture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.seen = append(c.seen, captured{
		host: r.Host, path: r.URL.Path, method: r.Method,
		signature: r.Header.Get("X-Lab-Signature"),
		namespace: r.Header.Get("X-Lab-Namespace"),
	})
	c.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.seen)
}

func (c *capture) requests() []captured {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]captured(nil), c.seen...)
}

func testServer(t *testing.T, resolver *Resolver, forwarder *Forwarder, token string) *httptest.Server {
	t.Helper()
	server := &Server{Resolver: resolver, Forwarder: forwarder, Token: token, Observer: NewObserver()}
	return httptest.NewServer(server.Handler())
}

func doRequest(t *testing.T, client *http.Client, method, url string, headers map[string]string) (int, []byte) {
	t.Helper()
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader("payload")
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, answer
}

func authHeaders() map[string]string {
	return map[string]string{"Authorization": "Bearer " + testToken}
}

func decisionFrom(t *testing.T, body []byte) Decision {
	t.Helper()
	var decision Decision
	if err := json.Unmarshal(body, &decision); err != nil {
		t.Fatalf("decode decision %q: %v", body, err)
	}
	return decision
}

// Scenario 1: a known name resolves to an agreed border address and namespace.
func TestKnownNameResolvesToAgreedRoute(t *testing.T) {
	srv := testServer(t, sampleResolver(t), &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=42", authHeaders())
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	decision := decisionFrom(t, body)
	if decision.Decision != "forward" || decision.Route == nil {
		t.Fatalf("decision = %+v, want forward", decision)
	}
	if decision.Route.Namespace != "session-42" || decision.Route.Address != testBorder {
		t.Fatalf("route = %+v, want the agreed namespace and border", decision.Route)
	}
}

// Scenario 2: an unknown name is quarantined without a route.
func TestUnknownNameQuarantined(t *testing.T) {
	srv := testServer(t, sampleResolver(t), &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=nowhere", authHeaders())
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	decision := decisionFrom(t, body)
	if decision.Decision != "quarantine" || decision.Reason != "unknown" || decision.Route != nil {
		t.Fatalf("decision = %+v, want quarantine/unknown with no route", decision)
	}
	if strings.Contains(string(body), `"address"`) || strings.Contains(string(body), `"namespace"`) {
		t.Fatalf("refusal leaked a route: %s", body)
	}
}

// Scenario 3: an ambiguous name is quarantined rather than guessed.
func TestAmbiguousNameQuarantined(t *testing.T) {
	raw := `{"agreements":[` +
		`{"id":"a","namespace":"ns-a","border":"http://a.invalid/cluster","aliases":["42"]},` +
		`{"id":"b","namespace":"ns-b","border":"http://b.invalid/cluster","aliases":["42"]}]}`
	resolver, err := LoadAgreements(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("load agreements: %v", err)
	}
	srv := testServer(t, resolver, &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=42", authHeaders())
	if status != http.StatusUnprocessableEntity || decisionFrom(t, body).Reason != "ambiguous" {
		t.Fatalf("status = %d, body = %s; want quarantine/ambiguous", status, body)
	}
}

// Scenario 4: an untrusted caller is refused before resolution.
func TestUntrustedCallerRefused(t *testing.T) {
	srv := testServer(t, sampleResolver(t), &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=42", nil)
	if status != http.StatusForbidden || decisionFrom(t, body).Reason != "untrusted" {
		t.Fatalf("status = %d, body = %s; want 403/untrusted", status, body)
	}
	if strings.Contains(string(body), `"address"`) {
		t.Fatalf("untrusted caller learned a route: %s", body)
	}
}

// Scenario 5: a malformed name is quarantined by the parser.
func TestMalformedNameQuarantined(t *testing.T) {
	srv := testServer(t, sampleResolver(t), &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=%00", authHeaders())
	if status != http.StatusUnprocessableEntity || decisionFrom(t, body).Reason != "malformed" {
		t.Fatalf("status = %d, body = %s; want quarantine/malformed", status, body)
	}
}

// Fuzz regression 617c154ecf3f62e2: "0 ." must canonicalize to the agreed "0",
// not to "0 " on the first read and "0" on the next.
func TestTrailingDotNameCanonicalizesToAgreedKey(t *testing.T) {
	raw := `{"agreements":[{"id":"workspace-0","namespace":"session-42","border":"` + testBorder + `","aliases":["0"]}]}`
	resolver, err := LoadAgreements(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("load agreements: %v", err)
	}
	srv := testServer(t, resolver, &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=0%20%2e", authHeaders())
	decision := decisionFrom(t, body)
	if status != http.StatusOK || decision.Decision != "forward" || decision.Route == nil || decision.Route.Namespace != "session-42" {
		t.Fatalf("status = %d, body = %s; want the agreed route for \"0\"", status, body)
	}
}

// Fuzz regression 5f8f3f6c02d5868c: "%25" must not become a canonical name
// that fails to reparse.
func TestEncodedPercentQuarantined(t *testing.T) {
	srv := testServer(t, sampleResolver(t), &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=%2525", authHeaders())
	if status != http.StatusUnprocessableEntity || decisionFrom(t, body).Reason != "malformed" {
		t.Fatalf("status = %d, body = %s; want quarantine/malformed", status, body)
	}
}

// Scenario 8: a known name reaches only the agreed border, with a signature.
func TestForwardReachesOnlyAgreedBorderSigned(t *testing.T) {
	border, attacker := &capture{}, &capture{}
	borderSrv, attackerSrv := httptest.NewServer(border), httptest.NewServer(attacker)
	defer borderSrv.Close()
	defer attackerSrv.Close()

	agreements := fmt.Sprintf(`{"agreements":[{"id":"workspace-42","namespace":"session-42","border":%q,"aliases":["42"]}]}`, borderSrv.URL)
	resolver, err := LoadAgreements(strings.NewReader(agreements))
	if err != nil {
		t.Fatalf("load agreements: %v", err)
	}
	srv := testServer(t, resolver, &Forwarder{BorderKey: []byte(testKey)}, testToken)
	defer srv.Close()

	headers := authHeaders()
	headers["X-Lab-Address"] = attackerSrv.URL
	status, body := doRequest(t, srv.Client(), http.MethodPost, srv.URL+"/forward?name=42", headers)
	if status != http.StatusOK || decisionFrom(t, body).Decision != "forward" {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if border.count() != 1 || attacker.count() != 0 {
		t.Fatalf("border got %d requests, attacker got %d", border.count(), attacker.count())
	}
	got := border.requests()[0]
	if !strings.HasPrefix(got.path, "/namespace/session-42/name/42") || got.namespace != "session-42" {
		t.Fatalf("border saw the wrong target: %+v", got)
	}
	if len(got.signature) != 64 {
		t.Fatalf("signature = %q, want 64 hex characters", got.signature)
	}
}

// Scenario 9: a known name is quarantined when no border key is configured.
func TestNoBorderKeyQuarantines(t *testing.T) {
	border := &capture{}
	borderSrv := httptest.NewServer(border)
	defer borderSrv.Close()

	agreements := fmt.Sprintf(`{"agreements":[{"id":"workspace-42","namespace":"session-42","border":%q,"aliases":["42"]}]}`, borderSrv.URL)
	resolver, err := LoadAgreements(strings.NewReader(agreements))
	if err != nil {
		t.Fatalf("load agreements: %v", err)
	}
	srv := testServer(t, resolver, &Forwarder{}, testToken)
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodPost, srv.URL+"/forward?name=42", authHeaders())
	if status != http.StatusUnprocessableEntity || decisionFrom(t, body).Reason != "border_unconfigured" {
		t.Fatalf("status = %d, body = %s; want quarantine/border_unconfigured", status, body)
	}
	if border.count() != 0 {
		t.Fatalf("border received %d requests without a key", border.count())
	}
}

// Scenario 10: health answers without any agreements.
func TestHealthWithoutConfig(t *testing.T) {
	srv := testServer(t, nil, nil, "")
	defer srv.Close()

	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/healthz", nil)
	if status != http.StatusOK || string(body) != "ok" {
		t.Fatalf("healthz = %d %q, want 200 ok", status, body)
	}
	if status, _ := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/readyz", nil); status != http.StatusServiceUnavailable {
		t.Fatalf("readyz = %d, want 503 with no config", status)
	}
}

// Scenario 11: a quarantine is visible on the status surface.
func TestQuarantineVisibleOnStatus(t *testing.T) {
	observer := NewObserver()
	server := &Server{Resolver: sampleResolver(t), Forwarder: &Forwarder{BorderKey: []byte(testKey)}, Token: testToken, Observer: observer}
	srv := httptest.NewServer(server.Handler())
	defer srv.Close()

	status, _ := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/resolve?name=nowhere", authHeaders())
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("quarantine status = %d", status)
	}
	status, body := doRequest(t, srv.Client(), http.MethodGet, srv.URL+"/status", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var counts statusBody
	if err := json.Unmarshal(body, &counts); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if counts.Quarantined["unknown"] != 1 {
		t.Fatalf("status = %+v, want one unknown quarantine", counts)
	}
}
