package workspaces

import (
	"context"
	"fmt"
	"strings"
)

// Seed materializes a spec in a store: the Gherkin is the origin, and this is its
// inversion (register — the Gherkin is the origin point on the plane).
//
// What the spec declares is what must exist. The MCP surface is files (§2), so a
// declaration is a file, and the spec names it. Nothing here executes a step:
// the spec says *what exists*, not what the session does — the state machine is
// the run's, not the seed's.
func Seed(ctx context.Context, s *Store, spec SpecRef) error {
	if err := s.Init(ctx); err != nil {
		return err
	}
	want, err := spec.Expectation()
	if err != nil {
		return err
	}
	decl, err := spec.Declarations()
	if err != nil {
		return err
	}

	// Write through the store's own writers — the MCP mapping *as file
	// operations* — so a declaration becomes exactly the file the store derives.
	for _, p := range decl.ByKind("resource_file") {
		if err := s.AddResource(ctx, strings.TrimPrefix(p, DirResources+"/"), []byte("{}\n")); err != nil {
			return err
		}
	}
	for _, name := range want.Tools {
		if err := s.AddTool(ctx, name, []byte("#!/bin/sh\n")); err != nil {
			return err
		}
	}
	for _, name := range want.Prompts {
		if err := s.AddPrompt(ctx, name, []byte("{{text}}\n")); err != nil {
			return err
		}
	}

	// Then check the two readings agree: the path the spec *names* must be the
	// path the store *derived*. A mismatch is a finding against the mapping, not
	// a silent drift.
	return CheckLayout(ctx, s, spec)
}

// CheckLayout asserts the store holds every file the spec names. It is the tie
// between the spec and the store: the spec's own step text is the layout, and
// the store's own conventions are the layout — they are the same layout.
func CheckLayout(ctx context.Context, s *Store, spec SpecRef) error {
	decl, err := spec.Declarations()
	if err != nil {
		return err
	}
	for _, kind := range []string{"resource_file", "tool_file", "prompt_file"} {
		for _, p := range decl.ByKind(kind) {
			ok, err := s.drv.Exists(ctx, p)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("seed: the spec names %s, the store does not hold it", p)
			}
		}
	}
	return nil
}

// BindTools registers a stand-in implementation for every tool the spec declares.
// The *file* is the declaration — a tool appears when `/tools/<name>@` does, and
// disappears when it goes (§2, dynamic commands) — and the function is the
// stand-in for the executable until the file is one.
func BindTools(srv *Server, spec SpecRef) error {
	want, err := spec.Expectation()
	if err != nil {
		return err
	}
	for _, name := range want.Tools {
		srv.RegisterTool(name, echoTool)
	}
	return nil
}

// echoTool is the stand-in: `tools/call` is a file write plus a result read (§2),
// so the stand-in hands back what it was given.
func echoTool(in []byte) []byte { return in }
