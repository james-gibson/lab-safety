package workspaces

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
)

// The on-disk conventions (register, minimal.feature §1). These paths are the
// contract: the same tree must appear on every host.
const (
	PathTask  = ".a2a/task.json"
	PathState = ".state/current"
	PathTrans = ".state/transitions.log"

	DirA2A       = ".a2a"
	DirState     = ".state"
	DirMessages  = "messages"
	DirArtifacts = "artifacts"
	DirResources = "context/resources"
	DirTools     = "context/tools"
	DirPrompts   = "context/prompts"
)

// Store is a workspace store for one session, materialised on a Driver. It is
// the file model: every MCP operation below is a file operation, and nothing
// else (register, minimal.feature §2).
type Store struct {
	drv     Driver
	session string
	rec     *recorder
}

// New returns a store for session on drv.
func New(drv Driver, session string) *Store { return &Store{drv: drv, session: session} }

// Driver is the host the store lives on.
func (s *Store) Driver() Driver { return s.drv }

// Session is the session id (the prototype runs a single session, "42").
func (s *Store) Session() string { return s.session }

// withRecorder returns the same store bound to an operation recorder, so a run
// can be canonicalised afterwards (the isotope record).
func (s *Store) withRecorder(r *recorder) *Store {
	c := *s
	c.rec = r
	return &c
}

func (s *Store) note(kind, p string) {
	if s.rec != nil {
		s.rec.add(kind, p)
	}
}

// Init lays down the documented tree (Rule §1). It is idempotent.
func (s *Store) Init(ctx context.Context) error {
	for _, d := range []string{DirA2A, DirState, DirMessages, DirArtifacts, DirResources, DirTools, DirPrompts} {
		if err := s.drv.Mkdir(ctx, d); err != nil {
			return err
		}
	}
	if err := s.drv.Put(ctx, PathTask, []byte(`{"session":"`+s.session+`","state":"SUBMITTED"}`+"\n")); err != nil {
		return err
	}
	s.note("init", PathTask)
	return s.transition(ctx, "SUBMITTED", "init")
}

// State returns ".state/current".
func (s *Store) State(ctx context.Context) (string, error) {
	b, err := s.drv.Get(ctx, PathState)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Tree lists the whole store.
func (s *Store) Tree(ctx context.Context) ([]string, error) { return s.drv.List(ctx, "") }

// transition moves the state machine and appends ".state/transitions.log"
// (Rule §3).
func (s *Store) transition(ctx context.Context, to, why string) error {
	from := ""
	if b, err := s.drv.Get(ctx, PathState); err == nil {
		from = strings.TrimSpace(string(b))
	}
	if from == to {
		return nil
	}
	if err := s.drv.Put(ctx, PathState, []byte(to+"\n")); err != nil {
		return err
	}
	s.note("state", PathState)
	prev := ""
	if b, err := s.drv.Get(ctx, PathTrans); err == nil {
		prev = string(b)
	}
	line := fmt.Sprintf("%s -> %s (%s)\n", from, to, why)
	if err := s.drv.Put(ctx, PathTrans, []byte(prev+line)); err != nil {
		return err
	}
	s.note("state", PathTrans)
	return nil
}

// WriteMessage writes "messages/{seq}.{kind}.json" (zero-padded, Rule §1) and
// moves SUBMITTED -> WORKING (Rule §3).
func (s *Store) WriteMessage(ctx context.Context, seq int, kind string, body []byte) error {
	p := fmt.Sprintf("%s/%08d.%s.json", DirMessages, seq, kind)
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("message", p)
	return s.transition(ctx, "WORKING", fmt.Sprintf("message %s", path.Base(p)))
}

// WriteArtifactPart writes one artifact part, "artifacts/{id}/parts/{seq}"
// (zero-padded, Rule §1).
func (s *Store) WriteArtifactPart(ctx context.Context, id string, seq int, body []byte) error {
	p := fmt.Sprintf("%s/%s/parts/%08d", DirArtifacts, id, seq)
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("artifact", p)
	return nil
}

// FinalizeArtifact writes the final artifact and moves WORKING -> COMPLETED
// (Rule §3).
func (s *Store) FinalizeArtifact(ctx context.Context, id string) error {
	p := fmt.Sprintf("%s/%s/final", DirArtifacts, id)
	if err := s.drv.Put(ctx, p, []byte("final\n")); err != nil {
		return err
	}
	s.note("artifact", p)
	return s.transition(ctx, "COMPLETED", "final artifact "+id)
}

// AddTool declares an executable tool endpoint, "context/tools/{name}@" (Rule §2).
func (s *Store) AddTool(ctx context.Context, name string, body []byte) error {
	p := DirTools + "/" + name + "@"
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("tool", p)
	return nil
}

// RemoveTool drops a tool at runtime (Rule §5).
func (s *Store) RemoveTool(ctx context.Context, name string) error {
	if err := s.drv.Delete(ctx, DirTools+"/"+name+"@"); err != nil {
		return err
	}
	s.note("tool", DirTools+"/"+name+"@")
	return nil
}

// AddResource declares a resource, "context/resources/{rel}" (Rule §2).
func (s *Store) AddResource(ctx context.Context, rel string, body []byte) error {
	p := DirResources + "/" + rel
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("resource", p)
	return nil
}

// AddPrompt declares a template, "context/prompts/{name}.tmpl" (Rule §2).
func (s *Store) AddPrompt(ctx context.Context, name string, body []byte) error {
	p := DirPrompts + "/" + name + ".tmpl"
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("prompt", p)
	return nil
}

// ---------- the MCP mapping: resources/tools/prompts are files ----------

// ResourcesList is resources/list: the URIs declared by files under
// context/resources, with the extension stripped (Rule §2).
func (s *Store) ResourcesList(ctx context.Context) ([]string, error) {
	ps, err := s.drv.List(ctx, DirResources)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range ps {
		if strings.HasSuffix(p, "/") {
			continue
		}
		rel := strings.TrimPrefix(p, DirResources+"/")
		rel = strings.TrimSuffix(rel, path.Ext(rel))
		out = append(out, "resource://"+rel)
	}
	sort.Strings(out)
	return out, nil
}

// ResourcesRead is resources/read: the bytes of the file behind a URI.
func (s *Store) ResourcesRead(ctx context.Context, uri string) ([]byte, error) {
	name := strings.TrimPrefix(uri, "resource://")
	ps, err := s.drv.List(ctx, DirResources)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if strings.HasSuffix(p, "/") {
			continue
		}
		rel := strings.TrimPrefix(p, DirResources+"/")
		rel = strings.TrimSuffix(rel, path.Ext(rel))
		if rel == name {
			return s.drv.Get(ctx, p)
		}
	}
	return nil, fmt.Errorf("resource %q: %w", uri, ErrNotExist)
}

// ToolsList is tools/list: the names of context/tools/*@ files (Rule §2).
func (s *Store) ToolsList(ctx context.Context) ([]string, error) {
	ps, err := s.drv.List(ctx, DirTools)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range ps {
		if strings.HasSuffix(p, "@") {
			out = append(out, path.Base(strings.TrimSuffix(p, "@")))
		}
	}
	sort.Strings(out)
	return out, nil
}

// CallTool is tools/call: a file write plus a result read, and no transport
// other than the file (Rule §2). The tool itself is a stand-in — a function
// that, like the executable "@ file, produces the result.
func (s *Store) CallTool(ctx context.Context, name string, input []byte, tool func([]byte) []byte) ([]byte, error) {
	reqP := DirTools + "/" + name + ".in"
	resP := DirTools + "/" + name + ".out"
	if err := s.drv.Put(ctx, reqP, input); err != nil {
		return nil, err
	}
	s.note("call", reqP)
	out := tool(input)
	if err := s.drv.Put(ctx, resP, out); err != nil {
		return nil, err
	}
	s.note("call", resP)
	return s.drv.Get(ctx, resP)
}

// PromptGet is prompts/get: the template file, rendered (Rule §2).
func (s *Store) PromptGet(ctx context.Context, name string, data map[string]string) (string, error) {
	b, err := s.drv.Get(ctx, DirPrompts+"/"+name+".tmpl")
	if err != nil {
		return "", err
	}
	out := string(b)
	for k, v := range data {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out, nil
}

// PromptList is prompts/list: the names of context/prompts/*.tmpl files (Rule §2).
func (s *Store) PromptList(ctx context.Context) ([]string, error) {
	ps, err := s.drv.List(ctx, DirPrompts)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range ps {
		if strings.HasSuffix(p, ".tmpl") {
			out = append(out, path.Base(strings.TrimSuffix(p, ".tmpl")))
		}
	}
	sort.Strings(out)
	return out, nil
}
