package workspaces

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// rpcCall is one request over the protocol, in process.
func rpcCall(t *testing.T, ctx context.Context, p *Protocol, body string) (json.RawMessage, *rpcError) {
	t.Helper()
	resp := p.Handle(ctx, []byte(body))
	var r struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	mustInto(t, resp, &r)
	return r.Result, r.Error
}

// subscribedStore is the smallest session the decoy can be asked about: a
// resource exists, so a subscribe can be *accepted* as well as refused.
func subscribedStore(t *testing.T, ctx context.Context, d Driver) *Store {
	t.Helper()
	s := New(d, "42")
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.AddResource(ctx, "workspace/config.json", []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestInitializeDoesNotPromiseDelivery: the surface states `subscribe: false`.
// That is what makes the decoy honest — a compliant client never subscribes, so
// anyone who does has gone past the promise (sheet §2, §7).
func TestInitializeDoesNotPromiseDelivery(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		p := NewProtocol(subscribedStore(t, ctx, d))

		res, rerr := rpcCall(t, ctx, p, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
		if rerr != nil {
			t.Fatal(rerr.Message)
		}
		var init struct {
			Capabilities map[string]map[string]any `json:"capabilities"`
		}
		mustInto(t, res, &init)

		resources, ok := init.Capabilities["resources"]
		if !ok {
			t.Fatal("the resources family is not advertised at all")
		}
		sub, ok := resources["subscribe"]
		if !ok {
			t.Error("resources.subscribe is omitted; state it false, so a caller cannot read silence as a promise")
		}
		if sub != false {
			t.Errorf("resources.subscribe = %v, want false — the surface delivers nothing", sub)
		}
		if resources["listChanged"] != false {
			t.Errorf("resources.listChanged = %v, want false", resources["listChanged"])
		}
	})
}

// TestSubscribeAnswersPlausiblyAndRecordsTheKnock is the decoy's contract: a
// real surface answers `{}`, so this one does too — and the knock lands in the
// session's own ledger, where nothing about the answer reveals it.
func TestSubscribeAnswersPlausiblyAndRecordsTheKnock(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		res, rerr := rpcCall(t, ctx, p,
			`{"jsonrpc":"2.0","id":2,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`)
		if rerr != nil {
			t.Fatalf("subscribe refused a URI the session holds: %s", rerr.Message)
		}
		if string(res) != "{}" {
			t.Fatalf("result = %s, want {} — a real surface answers nothing here", res)
		}

		cs, err := Contacts(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) != 1 {
			t.Fatalf("contacts = %d, want 1", len(cs))
		}
		c := cs[0]
		if c.Method != "resources/subscribe" || c.URI != "resource://workspace/config" {
			t.Errorf("contact = %+v, want the method and URI that were called", c)
		}
		if c.Outcome != OutcomeAccepted {
			t.Errorf("outcome = %q, want %q", c.Outcome, OutcomeAccepted)
		}
		if c.Advertised {
			t.Error("the contact is recorded as advertised; then it is not a decoy")
		}
		if c.Distance42i != Distance42i {
			t.Errorf("distance_42i = %d, want %d", c.Distance42i, Distance42i)
		}
		if c.Via != "method" {
			t.Errorf("via = %q, want method", c.Via)
		}
		if c.At == "" {
			t.Error("contact has no timestamp")
		}
	})
}

// TestSubscribeRefusesAURIItsNotHoldButStillRecordsIt: the decoy must not reveal
// itself on the first nonsense URI, so an unknown resource is refused exactly as
// a real surface refuses it — and the refusal is contact all the same.
func TestSubscribeRefusesAURIItsNotHoldButStillRecordsIt(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		_, rerr := rpcCall(t, ctx, p,
			`{"jsonrpc":"2.0","id":3,"method":"resources/subscribe","params":{"uri":"resource://workspace/nope"}}`)
		if rerr == nil || rerr.Code != -32602 {
			t.Fatalf("unknown URI: error = %+v, want -32602", rerr)
		}

		cs, err := Contacts(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) != 1 || cs[0].Outcome != OutcomeRefused {
			t.Fatalf("contacts = %+v, want one refused knock", cs)
		}
		if cs[0].URI != "resource://workspace/nope" {
			t.Errorf("contact URI = %q, want the URI that was refused", cs[0].URI)
		}
	})
}

// TestUnsubscribeIsTheOtherHalf: a caller that unsubscribes has believed it was
// subscribed. Same door, same ledger, and the append stays a ledger (two knocks,
// two lines, in order).
func TestUnsubscribeIsTheOtherHalf(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		for _, m := range []string{"resources/subscribe", "resources/unsubscribe"} {
			if _, rerr := rpcCall(t, ctx, p,
				`{"jsonrpc":"2.0","id":4,"method":"`+m+`","params":{"uri":"resource://workspace/config"}}`); rerr != nil {
				t.Fatalf("%s: %s", m, rerr.Message)
			}
		}

		cs, err := Contacts(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) != 2 {
			t.Fatalf("contacts = %d, want 2", len(cs))
		}
		if cs[0].Method != "resources/subscribe" || cs[1].Method != "resources/unsubscribe" {
			t.Errorf("ledger order = %q, %q — want subscribe then unsubscribe", cs[0].Method, cs[1].Method)
		}
	})
}

// TestTheDecoyDoesNotRevealItself: the response is byte-for-byte the shape a real
// surface gives. A decoy that says "recorded" in its answer is a tripwire the
// caller can see, which is no tripwire at all (TRIPWIRE.md: the decoy returns a
// plausible result "so that it does not reveal itself").
func TestTheDecoyDoesNotRevealItself(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		p := NewProtocol(subscribedStore(t, ctx, d))

		raw := p.Handle(ctx, []byte(
			`{"jsonrpc":"2.0","id":5,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`))

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatalf("response is not JSON: %s", raw)
		}
		if len(envelope) != 3 {
			t.Errorf("envelope keys = %v, want exactly jsonrpc, id, result", keysOf(envelope))
		}
		lower := strings.ToLower(string(raw))
		for _, leak := range []string{"honeypot", "contact", "decoy", "tripwire", "recorded"} {
			if strings.Contains(lower, leak) {
				t.Errorf("the answer names %q: %s", leak, raw)
			}
		}
	})
}

// TestTheLedgerNamesThePeer: over HTTP the contact says who knocked. An caller
// with no peer (in process) is recorded without one rather than with a fiction.
func TestTheLedgerNamesThePeer(t *testing.T) {
	ctx := context.Background()
	d, err := NewTmpDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	s := subscribedStore(t, ctx, d)

	ts := httptest.NewServer(NewServer(s))
	t.Cleanup(ts.Close)

	body := `{"jsonrpc":"2.0","id":6,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`
	resp, err := http.Post(ts.URL+"/mcp", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	cs, err := Contacts(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("contacts = %d, want 1", len(cs))
	}
	host, _, err := net.SplitHostPort(cs[0].Peer)
	if err != nil {
		t.Fatalf("peer = %q, want host:port: %v", cs[0].Peer, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("peer host = %q, want the loopback caller", host)
	}
}

// TestTheDecoyIsInvisibleUntilTouched: nothing is created by Init, and a normal
// read leaves no trace. The presence of the directory is itself evidence of a
// knock (TRIPWIRE.md: a decoy visible to the node is not a decoy).
func TestTheDecoyIsInvisibleUntilTouched(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		if ok, err := d.Exists(ctx, DirHoneypot); err != nil || ok {
			t.Fatalf("honeypot exists before any contact (ok=%v, err=%v)", ok, err)
		}
		if _, rerr := rpcCall(t, ctx, p, `{"jsonrpc":"2.0","id":7,"method":"resources/list"}`); rerr != nil {
			t.Fatal(rerr.Message)
		}
		if ok, err := d.Exists(ctx, DirHoneypot); err != nil || ok {
			t.Fatalf("a plain read created the honeypot (ok=%v, err=%v)", ok, err)
		}

		if _, rerr := rpcCall(t, ctx, p,
			`{"jsonrpc":"2.0","id":8,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`); rerr != nil {
			t.Fatal(rerr.Message)
		}
		if ok, err := d.Exists(ctx, DirHoneypot); err != nil || !ok {
			t.Fatalf("the knock left no trace (ok=%v, err=%v)", ok, err)
		}
	})
}

// TestSubscribeDeliversNothing is the boundary the honeypot pins. A subscriber
// may subscribe, the surface answers, and *nothing arrives* — not on a write to
// the resource, not ever. A green here is not A7 passing; it is A7's fast rung
// being absent, held so it cannot be mistaken for delivery.
func TestSubscribeDeliversNothing(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		if _, rerr := rpcCall(t, ctx, p,
			`{"jsonrpc":"2.0","id":9,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`); rerr != nil {
			t.Fatal(rerr.Message)
		}
		// The resource changes under the subscription: a real subscribe would
		// now send notifications/resources/updated. This one sends nothing.
		if err := s.AddResource(ctx, "workspace/config.json", []byte(`{"v":2}`)); err != nil {
			t.Fatal(err)
		}

		entries, err := d.List(ctx, DirHoneypot)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0] != PathContacts {
			t.Fatalf("honeypot holds %v, want only %s — delivery would have to write here first",
				entries, PathContacts)
		}
		cs, err := Contacts(ctx, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) != 1 {
			t.Fatalf("contacts = %d, want 1 — a delivery is not a contact", len(cs))
		}
	})
}

// TestNoSessionMeansNoKnock: with no session behind the surface every method is
// refused, and the honeypot must not manufacture state to record a caller it
// cannot answer. The refusal is on the wire; there is nowhere honest to write.
func TestNoSessionMeansNoKnock(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := subscribedStore(t, ctx, d)
		p := NewProtocol(s)

		if err := d.Delete(ctx, PathTask); err != nil {
			t.Fatal(err)
		}
		_, rerr := rpcCall(t, ctx, p,
			`{"jsonrpc":"2.0","id":10,"method":"resources/subscribe","params":{"uri":"resource://workspace/config"}}`)
		if rerr == nil || rerr.Code != -32001 {
			t.Fatalf("error = %+v, want -32001 (no session)", rerr)
		}
		if ok, err := d.Exists(ctx, DirHoneypot); err != nil || ok {
			t.Fatalf("a knock against no session created state (ok=%v, err=%v)", ok, err)
		}
	})
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
