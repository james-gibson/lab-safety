package workspaces

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// forEachHost runs the same assertions on every host. This is the point: the
// suite the sqlite host must pass is *unchanged* (register §5, Phase 2).
func forEachHost(t *testing.T, fn func(t *testing.T, d Driver)) {
	t.Helper()
	t.Run("tmpdir", func(t *testing.T) {
		d, err := NewTmpDir(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		fn(t, d)
	})
	t.Run("sqlite", func(t *testing.T) {
		d, err := NewSQLite(filepath.Join(t.TempDir(), "42.wsdb"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		fn(t, d)
	})
}

func filesOnly(ps []string) []string {
	out := []string{}
	for _, p := range ps {
		if !strings.HasSuffix(p, "/") {
			out = append(out, p)
		}
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// ---- Rule §1 — the on-disk conventions ----

func TestRule1Layout(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		tree, err := s.Tree(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			PathTask, PathState,
			DirMessages + "/", DirArtifacts + "/",
			DirResources + "/", DirTools + "/", DirPrompts + "/",
		} {
			if !contains(tree, want) {
				t.Errorf("tree missing %q\ngot %v", want, tree)
			}
		}
	})
}

func TestRule1SequenceNumbersOrderAsFiles(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		for _, seq := range []int{1, 2} {
			if err := s.WriteMessage(ctx, seq, "user", []byte(`{}`)); err != nil {
				t.Fatal(err)
			}
		}
		msgs, err := d.List(ctx, DirMessages)
		if err != nil {
			t.Fatal(err)
		}
		got := filesOnly(msgs)
		want := []string{"messages/00000001.user.json", "messages/00000002.user.json"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("messages = %v, want %v", got, want)
		}

		for _, seq := range []int{1, 2} {
			if err := s.WriteArtifactPart(ctx, "a1", seq, []byte("x")); err != nil {
				t.Fatal(err)
			}
		}
		parts, err := d.List(ctx, DirArtifacts+"/a1/parts")
		if err != nil {
			t.Fatal(err)
		}
		wantP := []string{"artifacts/a1/parts/00000001", "artifacts/a1/parts/00000002"}
		if !reflect.DeepEqual(parts, wantP) {
			t.Errorf("parts = %v, want %v", parts, wantP)
		}
	})
}

// ---- Rule §2 — the MCP mapping ----

func TestRule2ResourcesAreFiles(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		body := []byte(`{"k":1}` + "\n")
		if err := s.AddResource(ctx, "workspace/config.json", body); err != nil {
			t.Fatal(err)
		}

		uris, err := s.ResourcesList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(uris, "resource://workspace/config") {
			t.Errorf("resources/list = %v, want resource://workspace/config", uris)
		}
		got, err := s.ResourcesRead(ctx, "resource://workspace/config")
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(body) {
			t.Errorf("resources/read = %q, want %q", got, body)
		}

		// the set from `find context/resources` equals the set from resources/list
		found, err := d.List(ctx, DirResources)
		if err != nil {
			t.Fatal(err)
		}
		if len(filesOnly(found)) != len(uris) {
			t.Errorf("find gives %v, resources/list gives %v", filesOnly(found), uris)
		}
	})
}

func TestRule2ToolsAreFiles(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddTool(ctx, "echo", []byte("#!/bin/sh\n")); err != nil {
			t.Fatal(err)
		}

		names, err := s.ToolsList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(names, "echo") {
			t.Errorf("tools/list = %v, want echo", names)
		}

		found, err := d.List(ctx, DirTools)
		if err != nil {
			t.Fatal(err)
		}
		atFiles := 0
		for _, p := range found {
			if strings.HasSuffix(p, "@") {
				atFiles++
			}
		}
		if atFiles != len(names) {
			t.Errorf("ls *@ gives %d, tools/list gives %d", atFiles, len(names))
		}
	})
}

func TestRule2ToolCallIsAFileWritePlusResultRead(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddTool(ctx, "echo", []byte("#!/bin/sh\n")); err != nil {
			t.Fatal(err)
		}

		out, err := s.CallTool(ctx, "echo", []byte("hi"), func(in []byte) []byte {
			return append([]byte("echo:"), in...)
		})
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "echo:hi" {
			t.Errorf("result = %q, want %q", out, "echo:hi")
		}
		// the request was a file write, the result a file read: no other transport
		if _, err := d.Get(ctx, DirTools+"/echo.in"); err != nil {
			t.Errorf("request file missing: %v", err)
		}
		if b, err := d.Get(ctx, DirTools+"/echo.out"); err != nil || string(b) != "echo:hi" {
			t.Errorf("result file = %q (%v)", b, err)
		}
	})
}

func TestRule2PromptsAreTemplates(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddPrompt(ctx, "summarize", []byte("Summarize: {{text}}")); err != nil {
			t.Fatal(err)
		}
		got, err := s.PromptGet(ctx, "summarize", map[string]string{"text": "the store"})
		if err != nil {
			t.Fatal(err)
		}
		if got != "Summarize: the store" {
			t.Errorf("prompts/get = %q", got)
		}
	})
}

// ---- Rule §3 — the state machine is driven by file writes ----

func TestRule3MessageMovesSubmittedToWorking(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if st, _ := s.State(ctx); st != "SUBMITTED" {
			t.Fatalf("initial state = %q, want SUBMITTED", st)
		}
		if err := s.WriteMessage(ctx, 1, "user", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		if st, _ := s.State(ctx); st != "WORKING" {
			t.Errorf("state = %q, want WORKING", st)
		}
		log, err := d.Get(ctx, PathTrans)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), "SUBMITTED -> WORKING") {
			t.Errorf("transitions.log = %q, want SUBMITTED -> WORKING", log)
		}
	})
}

func TestRule3FinalArtifactMovesWorkingToCompleted(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.WriteMessage(ctx, 1, "user", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		if err := s.FinalizeArtifact(ctx, "a1"); err != nil {
			t.Fatal(err)
		}
		if st, _ := s.State(ctx); st != "COMPLETED" {
			t.Errorf("state = %q, want COMPLETED", st)
		}
		log, err := d.Get(ctx, PathTrans)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), "WORKING -> COMPLETED") {
			t.Errorf("transitions.log = %q, want WORKING -> COMPLETED", log)
		}
	})
}

// ---- Rule §5 — commands are dynamic ----

func TestRule5ToolDropInAndRemoval(t *testing.T) {
	forEachHost(t, func(t *testing.T, d Driver) {
		ctx := context.Background()
		s := New(d, "42")
		if err := s.Init(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.AddTool(ctx, "late", []byte("#!/bin/sh\n")); err != nil {
			t.Fatal(err)
		}
		names, _ := s.ToolsList(ctx)
		if !contains(names, "late") {
			t.Errorf("dropped tool not listable: %v", names)
		}
		if err := s.RemoveTool(ctx, "late"); err != nil {
			t.Fatal(err)
		}
		names, _ = s.ToolsList(ctx)
		if contains(names, "late") {
			t.Errorf("removed tool still listed: %v", names)
		}
	})
}
