package workspaces

import (
	"context"
	"encoding/json"
	"testing"
)

func mustInto(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
}

// gateSpec is the expectation, read from the Gherkin (the spec is the origin).
func gateSpec(t *testing.T) Expectation {
	t.Helper()
	want, err := GateSpec()
	if err != nil {
		t.Fatalf("read the gate %s: %v", GatePath, err)
	}
	return want
}

func TestProtocolSpeaksMCP(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddTool(ctx, "echo", []byte("#!/bin/sh\n")); err != nil {
			t.Fatal(err)
		}
		p := NewProtocol(s)
		p.RegisterTool("echo", func(in []byte) []byte { return append([]byte("echo:"), in...) })

		var init struct {
			Result struct {
				ProtocolVersion string         `json:"protocolVersion"`
				ServerInfo      map[string]any `json:"serverInfo"`
			} `json:"result"`
		}
		mustInto(t, p.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)), &init)
		if init.Result.ProtocolVersion != ProtocolVersion {
			t.Errorf("protocolVersion = %q, want %q", init.Result.ProtocolVersion, ProtocolVersion)
		}
		if init.Result.ServerInfo["name"] != "workspace-42" {
			t.Errorf("serverInfo.name = %v", init.Result.ServerInfo["name"])
		}

		tools, err := listVia(ctx, p, "tools/list", "tools")
		if err != nil {
			t.Fatal(err)
		}
		if !contains(tools, "echo") {
			t.Errorf("tools/list = %v, want echo", tools)
		}

		var call struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		mustInto(t, p.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":"hi"}}`)), &call)
		if len(call.Result.Content) != 1 || call.Result.Content[0].Text != `echo:"hi"` {
			t.Errorf("tools/call content = %+v", call.Result.Content)
		}

		var bad struct {
			Error *rpcError `json:"error"`
		}
		mustInto(t, p.Handle(ctx, []byte(`{"jsonrpc":"2.0","id":3,"method":"nope"}`)), &bad)
		if bad.Error == nil || bad.Error.Code != -32601 {
			t.Errorf("unknown method: %+v, want -32601", bad.Error)
		}
	})
}

// TestVerificationIsIndependent is the point of the step: the expectation comes
// from the spec (not the run), and the observation is read back through the
// protocol from the mirror (a different position than the write path).
func TestVerificationIsIndependent(t *testing.T) {
	ctx := context.Background()
	c, _, sq := newClient(t)

	if _, err := c.Run(ctx, "42", func(s *Store) error { return prog(ctx, s) }); err != nil {
		t.Fatal(err)
	}

	got, err := Observe(ctx, sq, "42")
	if err != nil {
		t.Fatal(err)
	}
	if mis := Verify(gateSpec(t), got); len(mis) != 0 {
		t.Fatalf("spec mismatch = %v (observed %+v)", mis, got)
	}

	rep, err := c.Report(ctx, "42", gateSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Agrees() {
		t.Fatalf("expected agreement, got %+v", rep)
	}

	// verified from a different position -> the pair is now independent: -42 green
	p := Assess(rep, true, true)
	if p != PlaneWork {
		t.Errorf("plane = %s, want %s", p, PlaneWork)
	}
	if p.Light() != "green" {
		t.Errorf("light = %s, want green", p.Light())
	}
}

// TestVerificationCatchesADivergentMirror shows the second measurement is not
// the writer's own record: the mirror is read back, and a divergence is torn.
func TestVerificationCatchesADivergentMirror(t *testing.T) {
	ctx := context.Background()
	c, _, sq := newClient(t)

	if _, err := c.Run(ctx, "42", func(s *Store) error { return prog(ctx, s) }); err != nil {
		t.Fatal(err)
	}

	// the mirror diverges from the spec: a tool the spec expects is gone
	if err := sq.Delete(ctx, DirTools+"/echo@"); err != nil {
		t.Fatal(err)
	}

	rep, err := c.Report(ctx, "42", gateSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Torn() {
		t.Fatalf("expected a torn report, got %+v", rep)
	}
	if len(rep.SpecMismatch) != 1 || rep.SpecMismatch[0] != "tools" {
		t.Errorf("spec mismatch = %v, want [tools]", rep.SpecMismatch)
	}
	if p := Assess(rep, true, true); p != PlaneTorn || p.Light() != "red" {
		t.Errorf("plane = %s light = %s, want torn/red", p, p.Light())
	}
}
