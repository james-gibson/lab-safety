package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// A honeypot is a door that should not be knocked on: it answers plausibly, and
// the *contact* is the finding (lab/TRIPWIRE.md). This surface cannot keep the
// promise `resources/subscribe` makes — the session pushes nothing, because
// delivery is A7's other half and it does not exist yet — so the method is not
// advertised, and answering it anyway is the decoy: a caller that subscribes has
// gone past what the surface offered, and the knock is recorded.
//
// The decoy is not a false declaration. `initialize` says `subscribe: false`
// (protocol.go, availableCapabilities), so a compliant client never calls it; a
// caller that does is probing, which is exactly what the sheet's hazard class
// `H` means. The difference between this and the declaration the SDS exists to
// catch is on the wire, not in the comment.
//
// What the surface does *not* do is judge the contact. It records the knock; the
// verdict belongs to a detector outside the honeypotted node (TRIPWIRE.md), and
// no detector reads this ledger yet (sheet §11, §16 finding B).

const (
	// DirHoneypot is where contacts land, under the session's own state. It is
	// created by the first contact, not by Init: a decoy that is visible before
	// anyone touches it is not a decoy.
	DirHoneypot = "/state/honeypot"
	// PathContacts is the contact ledger, append-only JSONL.
	PathContacts = DirHoneypot + "/contacts.jsonl"

	// Distance42i is the cost of a *direct method call* the surface never offered
	// — TRIPWIRE.md's +32 (the unknown-tool case; a tools/call at a decoy is
	// +16). The cost is the caller's. This surface only records it; accumulating
	// it across contacts and comparing it to a sensitivity is the detector's job
	// (declared, not implemented — sheet §11).
	Distance42i = 32
)

// Outcomes a contact can have. The outcome is what the surface *did*, not what
// the caller earned: a refusal is still contact, and a caller probing with a URI
// that does not exist is not less of a prober than one that guessed right.
const (
	OutcomeAccepted  = "accepted"
	OutcomeRefused   = "refused"
	OutcomeMalformed = "malformed"
)

// Contact is one knock on the decoy — who asked, for what, and what the surface
// answered. It is JSONL in PathContacts so a detector can read it without this
// package, and so the ledger is the session's state like everything else.
type Contact struct {
	At          string `json:"at"`
	Method      string `json:"method"`
	URI         string `json:"uri,omitempty"`
	Outcome     string `json:"outcome"`
	Via         string `json:"via"`            // method: a name invoked, not a tools/call
	Peer        string `json:"peer,omitempty"` // the HTTP caller, when there was one
	Advertised  bool   `json:"advertised"`     // what initialize promised (false: it did not)
	Distance42i int    `json:"distance_42i"`
}

// Contacts reads the ledger, oldest first. No contacts yet is an empty ledger,
// not an error — the absence is the normal state, and a reader must not have to
// distinguish "nothing knocked" from "the honeypot is broken".
func Contacts(ctx context.Context, s *Store) ([]Contact, error) {
	b, err := s.drv.Get(ctx, PathContacts)
	if errors.Is(err, ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Contact
	for _, line := range splitLines(b) {
		if len(line) == 0 {
			continue
		}
		var c Contact
		if err := json.Unmarshal(line, &c); err != nil {
			return nil, fmt.Errorf("contact ledger %s: %w", PathContacts, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// recordContact appends one knock. The append is a read-modify-write: the
// prototype runs one writer (register §2), and Protocol serialises its own
// calls, so an interleaved knock would be a finding rather than a silent pass.
func recordContact(ctx context.Context, s *Store, c Contact) error {
	if c.At == "" {
		c.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if c.Via == "" {
		c.Via = "method"
	}
	if c.Peer == "" {
		c.Peer = peerFrom(ctx)
	}
	if c.Distance42i == 0 {
		c.Distance42i = Distance42i
	}
	line, err := json.Marshal(c)
	if err != nil {
		return err
	}
	b, err := s.drv.Get(ctx, PathContacts)
	if err != nil && !errors.Is(err, ErrNotExist) {
		return err
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	b = append(b, line...)
	b = append(b, '\n')
	return s.drv.Put(ctx, PathContacts, b)
}

// subscribe answers the pair the surface never advertised. It answers exactly as
// a real surface would — `{}` for a URI the session holds, `-32602` for one it
// does not — because a decoy that always says yes reveals itself on the first
// nonsense URI. Nothing is delivered, and nothing says so: the promise was
// withheld in `initialize`, not broken here.
func (p *Protocol) subscribe(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	var prm struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &prm); err != nil {
		// A malformed subscribe is still contact. Record it, then answer as a
		// real surface does.
		if err := p.record(ctx, Contact{Method: method, Outcome: OutcomeMalformed}); err != nil {
			return nil, internal(err)
		}
		return nil, &rpcError{Code: -32602, Message: "invalid params"}
	}

	outcome := OutcomeAccepted
	if _, err := p.store.ResourcesRead(ctx, prm.URI); err != nil {
		outcome = OutcomeRefused
	}
	if err := p.record(ctx, Contact{Method: method, URI: prm.URI, Outcome: outcome}); err != nil {
		return nil, internal(err)
	}
	if outcome == OutcomeRefused {
		return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("resource %q: not found", prm.URI)}
	}
	return map[string]any{}, nil
}

// record serialises the ledger within this surface. Two processes writing the
// same session is out of scope (A13: one session, one writer).
func (p *Protocol) record(ctx context.Context, c Contact) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return recordContact(ctx, p.store, c)
}

// splitLines splits a JSONL file without copying the common case of a trailing
// newline into an empty record.
func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}
