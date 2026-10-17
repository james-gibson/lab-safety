package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Expectation is what should have happened: declared from the spec, *before* the
// run, and independently of it.
type Expectation struct {
	Tools     []string
	Resources []string
	Prompts   []string
}

// Spec is the expectation for the specimen run — the step definitions for
// minimal.feature §2's scenarios. It is deliberately a fixed declaration rather
// than something derived from the run: that is where the independence comes
// from. (A Gherkin parser replaces this declaration later.)
func Spec() Expectation {
	return Expectation{
		Tools:     []string{"echo"},
		Resources: []string{"resource://workspace/config"},
		Prompts:   []string{"summarize"},
	}
}

// Observe takes the second measurement from a *different position*: it reads a
// host back through the MCP protocol — the reader path, not the writer's driver
// calls, and (with the mirror) not the host written first.
func Observe(ctx context.Context, d Driver, session string) (Expectation, error) {
	p := NewProtocol(New(d, session))
	var e Expectation
	var err error
	if e.Tools, err = listVia(ctx, p, "tools/list", "tools"); err != nil {
		return e, err
	}
	if e.Resources, err = listVia(ctx, p, "resources/list", "resources"); err != nil {
		return e, err
	}
	if e.Prompts, err = listVia(ctx, p, "prompts/list", "prompts"); err != nil {
		return e, err
	}
	return e, nil
}

func listVia(ctx context.Context, p *Protocol, method, key string) ([]string, error) {
	resp := p.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"`+method+`"}`))
	var r struct {
		Result map[string][]map[string]any `json:"result"`
		Error  *rpcError                   `json:"error"`
	}
	if err := json.Unmarshal(resp, &r); err != nil {
		return nil, err
	}
	if r.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, r.Error.Message)
	}
	out := []string{}
	for _, item := range r.Result[key] {
		if v, ok := item["uri"].(string); ok {
			out = append(out, v)
			continue
		}
		if v, ok := item["name"].(string); ok {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Verify compares what should have happened (the spec) with what the host holds
// (the observation). The surfaces that disagree are the torn surface.
func Verify(want, got Expectation) []string {
	var out []string
	for _, f := range []struct {
		name      string
		want, got []string
	}{
		{"tools", want.Tools, got.Tools},
		{"resources", want.Resources, got.Resources},
		{"prompts", want.Prompts, got.Prompts},
	} {
		if !equalStrings(f.want, f.got) {
			out = append(out, f.name)
		}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Verification is the independent measurement: the mirror, read back through the
// MCP protocol, against the spec.
func (c *Client) Verification(ctx context.Context, session string, want Expectation) ([]string, error) {
	got, err := Observe(ctx, c.mirror, session)
	if err != nil {
		return nil, err
	}
	return Verify(want, got), nil
}

// Report runs both cross-checks: the writer's own claim (Check) and the
// independent observation (Verification).
func (c *Client) Report(ctx context.Context, session string, want Expectation) (Report, error) {
	rep, err := c.Check(ctx)
	if err != nil {
		return rep, err
	}
	if rep.SpecMismatch, err = c.Verification(ctx, session, want); err != nil {
		return rep, err
	}
	return rep, nil
}
