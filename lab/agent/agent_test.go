package agent

import (
	"context"
	"strings"
	"testing"

	acp1 "github.com/ironpark/acp-go/acp1"

	"github.com/james-gibson/lab-safety/workspaces"
)

// newPipe connects an in-process client to the agent, so the suite drives the
// same ACP surface an editor does without spawning a child.
func newPipe(t *testing.T) *acp1.ClientSideConnection {
	t.Helper()
	ctx := context.Background()

	drv, err := workspaces.NewTmpDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = drv.Close() })

	s := workspaces.New(drv, "42")
	if err := s.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTool(ctx, "echo", []byte("#!/bin/sh\n")); err != nil {
		t.Fatal(err)
	}
	if err := s.AddResource(ctx, "config", []byte("{}\n")); err != nil {
		t.Fatal(err)
	}

	_, remote := acp1.Pipe(ctx,
		func(c *acp1.AgentSideConnection) acp1.Agent {
			a := New(c, s, drv.Name())
			a.RegisterTool("echo", func(in []byte) []byte { return in })
			return a
		},
		func(*acp1.ClientSideConnection) acp1.Client { return acp1.UnimplementedClient{} },
	)
	return remote
}

func TestPromptIsACommandAgainstTheStore(t *testing.T) {
	ctx := context.Background()
	remote := newPipe(t)

	if _, err := remote.Initialize(ctx, &acp1.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}
	session, err := remote.StartSession(ctx, &acp1.NewSessionRequest{Cwd: "/somewhere/else"})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		prompt string
		want   string
	}{
		{"ls /", workspaces.DirData},
		{"tools", "echo"},
		{"resources", "config"},
		{"state", "SUBMITTED"},
		{"session", "session=42"},
		{`call echo {"a":1}`, `{"a":1}`},
	}
	for _, tc := range cases {
		turn, err := session.Prompt(ctx, acp1.TextBlock(tc.prompt))
		if err != nil {
			t.Fatalf("%q: %v", tc.prompt, err)
		}
		text, _, err := turn.Text()
		if err != nil {
			t.Fatalf("%q: %v", tc.prompt, err)
		}
		if !strings.Contains(text, tc.want) {
			t.Errorf("%q: got %q, want it to contain %q", tc.prompt, text, tc.want)
		}
	}
}

func TestWriteGoesThroughTheStoreWriter(t *testing.T) {
	ctx := context.Background()
	remote := newPipe(t)

	if _, err := remote.Initialize(ctx, &acp1.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}
	session, err := remote.StartSession(ctx, &acp1.NewSessionRequest{})
	if err != nil {
		t.Fatal(err)
	}

	turn, err := session.Prompt(ctx, acp1.TextBlock("write /resources/notes hello"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := turn.Text(); err != nil {
		t.Fatal(err)
	}

	turn, err = session.Prompt(ctx, acp1.TextBlock("resources"))
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := turn.Text()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "notes") {
		t.Errorf("resources after write = %q, want it to contain %q", text, "notes")
	}
}

// A path above the root is refused by the driver, not by a permission check:
// the session's isolation is structural (filesystem.feature, Rule 1).
func TestPathAboveTheRootIsRefused(t *testing.T) {
	ctx := context.Background()
	remote := newPipe(t)

	if _, err := remote.Initialize(ctx, &acp1.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}
	session, err := remote.StartSession(ctx, &acp1.NewSessionRequest{})
	if err != nil {
		t.Fatal(err)
	}

	turn, err := session.Prompt(ctx, acp1.TextBlock("cat /../etc/passwd"))
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := turn.Text()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "error:") {
		t.Errorf("cat /../etc/passwd = %q, want an error", text)
	}
}
