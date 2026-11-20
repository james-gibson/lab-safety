package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const fuzzBorder = "http://127.0.0.1:19100/cluster"

func fuzzResolver(t *testing.T) *Resolver {
	t.Helper()
	res, err := LoadAgreements(strings.NewReader(
		`{"agreements":[{"id":"workspace-42","namespace":"session-42","border":"` + fuzzBorder + `","aliases":["42"]}]}`))
	if err != nil {
		t.Fatalf("load fuzz agreements: %v", err)
	}
	return res
}

// FuzzResolveName: every input maps to a route from the agreement, or to a
// typed refusal with no route. It is total, deterministic and non-synthesising.
func FuzzResolveName(f *testing.F) {
	for _, seed := range []string{"42", "WS42", "nowhere", "localhost", "0.0.0.0", "::1", "a/../b", "%2e%2e", "%00", "", strings.Repeat("a", MaxNameBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		first := fuzzResolver(t).Resolve(raw)
		second := fuzzResolver(t).Resolve(raw)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("resolution is not deterministic: %+v / %+v", first, second)
		}
		switch first.Decision {
		case "forward":
			if first.Reason != "" || first.Route == nil {
				t.Fatalf("forward without a clean route: %+v", first)
			}
			if first.Route.Address != fuzzBorder || first.Route.Namespace != "session-42" {
				t.Fatalf("route was synthesised: %+v", first.Route)
			}
		case "quarantine":
			if first.Reason == "" || first.Route != nil {
				t.Fatalf("refusal without a typed reason or with a route: %+v", first)
			}
		default:
			t.Fatalf("unknown decision %q", first.Decision)
		}
	})
}

// FuzzQuarantineContract: a refusal always has a reason, carries no route, and
// lands in exactly one counter bucket on the status surface.
func FuzzQuarantineContract(f *testing.F) {
	for _, seed := range []string{"nowhere", "%00", "42", "..", "ambiguous"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		decision := fuzzResolver(t).Resolve(raw)
		if decision.Decision != "quarantine" {
			return // the forwarding contract is covered by FuzzBorderForward
		}
		if decision.Reason == "" || decision.Route != nil {
			t.Fatalf("bad quarantine: %+v", decision)
		}
		observer := NewObserver()
		observer.Record(decision)
		recorder := httptest.NewRecorder()
		observer.ServeStatus(false, recorder, httptest.NewRequest(http.MethodGet, "/status", nil))
		var counts statusBody
		if err := json.Unmarshal(recorder.Body.Bytes(), &counts); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if counts.Quarantined[decision.Reason] != 1 {
			t.Fatalf("status %+v does not show one %s quarantine", counts, decision.Reason)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

// FuzzBorderForward: even when a caller supplies a destination header, the
// only request that can be emitted goes to the agreed border, signed.
func FuzzBorderForward(f *testing.F) {
	f.Add("42", "http://attacker.invalid")
	f.Add("nowhere", "http://attacker.invalid")
	f.Add("%00", "http://attacker.invalid")
	f.Fuzz(func(t *testing.T, rawName, callerAddress string) {
		if len(rawName) > 1024 || len(callerAddress) > 1024 {
			t.Skip()
		}
		var mu sync.Mutex
		type sent struct{ target, signature string }
		var captured []sent
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			captured = append(captured, sent{target: req.URL.String(), signature: req.Header.Get("X-Lab-Signature")})
			mu.Unlock()
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK",
				Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req,
			}, nil
		})}
		server := &Server{
			Resolver:  fuzzResolver(t),
			Forwarder: &Forwarder{Client: client, BorderKey: []byte("secret")},
			Token:     "token",
			Observer:  NewObserver(),
		}
		req := httptest.NewRequest(http.MethodPost, "/forward?name="+url.QueryEscape(rawName), strings.NewReader("payload"))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("X-Lab-Address", callerAddress)
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)

		mu.Lock()
		seen := append([]sent(nil), captured...)
		mu.Unlock()
		if recorder.Code == http.StatusOK {
			if len(seen) != 1 {
				t.Fatalf("expected one forwarded request, got %d", len(seen))
			}
			if !strings.HasPrefix(seen[0].target, fuzzBorder+"/namespace/session-42/name/") {
				t.Fatalf("destination escaped the agreement: %q", seen[0].target)
			}
			if len(seen[0].signature) != 64 {
				t.Fatalf("request was not signed: %q", seen[0].signature)
			}
			return
		}
		if len(seen) != 0 {
			t.Fatalf("forwarded a quarantined request: %+v", seen)
		}
	})
}

// FuzzCanonicalize: the parser never truncates, invents structure, or changes
// a canonical name on a second pass.
func FuzzCanonicalize(f *testing.F) {
	for _, seed := range []string{"42", "%34%32", "WS42", "a/b", string([]byte{0}), strings.Repeat("a", MaxNameBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			t.Skip()
		}
		canonical, err := ParseName(raw)
		if err != nil {
			return
		}
		if canonical == "" || len(canonical) > MaxNameBytes || canonical != strings.ToLower(canonical) ||
			strings.ContainsAny(canonical, "/\\") || strings.Contains(canonical, "..") || hasControl(canonical) {
			t.Fatalf("non-canonical name %q from %q", canonical, raw)
		}
		again, err := ParseName(canonical)
		if err != nil || again != canonical {
			t.Fatalf("ParseName is not idempotent: %q -> %q (%v)", canonical, again, err)
		}
	})
}

// FuzzConfig: arbitrary agreement bytes either fail loudly or produce routes
// whose addresses are usable http(s) URLs; no key means no forwarding.
func FuzzConfig(f *testing.F) {
	f.Add([]byte(`{"agreements":[]}`))
	f.Add([]byte(testAgreement))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		resolver, err := LoadAgreements(bytes.NewReader(data))
		if err != nil {
			return
		}
		for _, name := range []string{"42", "nowhere", "%00"} {
			decision := resolver.Resolve(name)
			if decision.Decision == "forward" {
				if decision.Route == nil || decision.Route.Address == "" || decision.Route.Namespace == "" {
					t.Fatalf("incomplete route: %+v", decision)
				}
				u, err := url.Parse(decision.Route.Address)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					t.Fatalf("unusable border address: %q", decision.Route.Address)
				}
				continue
			}
			if decision.Reason == "" || decision.Route != nil {
				t.Fatalf("bad refusal: %+v", decision)
			}
		}
		if _, err := (&Forwarder{}).Forward(context.Background(),
			Route{Address: fuzzBorder, Namespace: "session-42"}, "42", []byte("x")); err != ErrBorderUnconfigured {
			t.Fatalf("forward without a key returned %v", err)
		}
	})
}
