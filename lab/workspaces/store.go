package workspaces

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
)

// The session IS the root filesystem (filesystem.feature). Paths are absolute in
// the session's namespace, and the session's definitions sit in "/".
const (
	// Root is the session. There is nothing above it.
	Root = "/"
	// Home is the session's home — *under* the root, not the root itself.
	Home = "/root"

	// data: what the session works on
	DirData      = "/data"
	DirMessages  = "/data/messages"
	DirArtifacts = "/data/artifacts"
	PathTask     = "/data/task.json"

	// state: where the session is
	DirState  = "/state"
	PathState = "/state/current"
	PathTrans = "/state/transitions.log"

	// MCP: the session's tools, resources and prompts
	DirTools     = "/tools"
	DirResources = "/resources"
	DirPrompts   = "/prompts"
)

type mount struct {
	dst      string // a mount point, a directory path ending in "/"
	from     *Store
	fromPath string // a directory path ending in "/"
}

// Store is one session's filesystem, materialised on a Driver. Every MCP
// operation below is a file operation, and nothing else (minimal.feature §2).
type Store struct {
	drv     Driver
	session string
	rec     *recorder
	mounts  []mount
}

// New returns a store for session on drv.
func New(drv Driver, session string) *Store { return &Store{drv: drv, session: session} }

// Driver is the host the session lives on.
func (s *Store) Driver() Driver { return s.drv }

// Session is the session id (the prototype runs a single session, "42").
func (s *Store) Session() string { return s.session }

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

// Init lays down the documented tree (Rule §1): the session's data, state and
// MCP surfaces in "/", and its home under it.
func (s *Store) Init(ctx context.Context) error {
	for _, d := range []string{
		Root, Home,
		DirData, DirMessages, DirArtifacts,
		DirState,
		DirTools, DirResources, DirPrompts,
	} {
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

// State returns "/state/current".
func (s *Store) State(ctx context.Context) (string, error) {
	b, err := s.drv.Get(ctx, PathState)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Tree lists the session's filesystem from the root.
func (s *Store) Tree(ctx context.Context) ([]string, error) { return s.drv.List(ctx, Root) }

// transition moves the state machine and appends "/state/transitions.log" (§3).
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

// WriteMessage writes "/data/messages/{seq}.{kind}.json" (zero-padded, §1) and
// moves SUBMITTED -> WORKING (§3).
func (s *Store) WriteMessage(ctx context.Context, seq int, kind string, body []byte) error {
	p := fmt.Sprintf("%s/%08d.%s.json", DirMessages, seq, kind)
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("message", p)
	return s.transition(ctx, "WORKING", fmt.Sprintf("message %s", path.Base(p)))
}

// WriteArtifactPart writes one artifact part, "/data/artifacts/{id}/parts/{seq}"
// (zero-padded, §1).
func (s *Store) WriteArtifactPart(ctx context.Context, id string, seq int, body []byte) error {
	p := fmt.Sprintf("%s/%s/parts/%08d", DirArtifacts, id, seq)
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("artifact", p)
	return nil
}

// FinalizeArtifact writes the final artifact and moves WORKING -> COMPLETED (§3).
func (s *Store) FinalizeArtifact(ctx context.Context, id string) error {
	p := fmt.Sprintf("%s/%s/final", DirArtifacts, id)
	if err := s.drv.Put(ctx, p, []byte("final\n")); err != nil {
		return err
	}
	s.note("artifact", p)
	return s.transition(ctx, "COMPLETED", "final artifact "+id)
}

// AddTool declares an executable tool endpoint, "/tools/{name}@" (§2).
func (s *Store) AddTool(ctx context.Context, name string, body []byte) error {
	p := DirTools + "/" + name + "@"
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("tool", p)
	return nil
}

// RemoveTool drops a tool at runtime (§5).
func (s *Store) RemoveTool(ctx context.Context, name string) error {
	p := DirTools + "/" + name + "@"
	if err := s.drv.Delete(ctx, p); err != nil {
		return err
	}
	s.note("tool", p)
	return nil
}

// AddResource declares a resource, "/resources/{rel}" (§2).
func (s *Store) AddResource(ctx context.Context, rel string, body []byte) error {
	p := DirResources + "/" + rel
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("resource", p)
	return nil
}

// AddPrompt declares a template, "/prompts/{name}.tmpl" (§2).
func (s *Store) AddPrompt(ctx context.Context, name string, body []byte) error {
	p := DirPrompts + "/" + name + ".tmpl"
	if err := s.drv.Put(ctx, p, body); err != nil {
		return err
	}
	s.note("prompt", p)
	return nil
}

// ---------- sharing: a session may join another session's path ----------

// Join joins another session's directory at dst — a mount point in this
// session's tree (filesystem.feature → Rule: sessions may share). The sharing is
// a join, not inheritance: nothing is copied, and a read follows the join.
func (s *Store) Join(ctx context.Context, dst string, from *Store, fromPath string) error {
	dk, err := dirKey(dst)
	if err != nil {
		return err
	}
	fk, err := dirKey(fromPath)
	if err != nil {
		return err
	}
	if err := s.drv.Mkdir(ctx, dk); err != nil {
		return err
	}
	s.mounts = append(s.mounts, mount{dst: dk, from: from, fromPath: fk})
	s.note("mount", dk)
	return nil
}

// resolve maps a path to the store and path it actually lives on, following any
// join. Everything else resolves to itself.
func (s *Store) resolve(p string) (*Store, string) {
	dp, err := dirKey(p)
	if err != nil {
		return s, p
	}
	for _, m := range s.mounts {
		if strings.HasPrefix(dp, m.dst) {
			rp := m.fromPath + strings.TrimPrefix(dp, m.dst)
			if !strings.HasSuffix(p, "/") {
				rp = strings.TrimSuffix(rp, "/")
			}
			return m.from, rp
		}
	}
	return s, p
}

// Read reads a path, following any join.
func (s *Store) Read(ctx context.Context, p string) ([]byte, error) {
	st, rp := s.resolve(p)
	return st.drv.Get(ctx, rp)
}

// ---------- the MCP mapping: resources/tools/prompts are files ----------

// ResourcesList is resources/list: the URIs declared by files under /resources,
// with the extension stripped (§2).
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
		out = append(out, resourceURI(p))
	}
	sort.Strings(out)
	return out, nil
}

// ResourcesRead is resources/read: the bytes of the file behind a URI.
func (s *Store) ResourcesRead(ctx context.Context, uri string) ([]byte, error) {
	ps, err := s.drv.List(ctx, DirResources)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if strings.HasSuffix(p, "/") {
			continue
		}
		if resourceURI(p) == uri {
			return s.Read(ctx, p)
		}
	}
	return nil, fmt.Errorf("resource %q: %w", uri, ErrNotExist)
}

// ToolsList is tools/list: the names of /tools/*@ files (§2).
func (s *Store) ToolsList(ctx context.Context) ([]string, error) {
	ps, err := s.drv.List(ctx, DirTools)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range ps {
		if strings.HasSuffix(p, "@") {
			out = append(out, toolName(p))
		}
	}
	sort.Strings(out)
	return out, nil
}

// CallTool is tools/call: a file write plus a result read, and no transport
// other than the file (§2). The tool itself is a stand-in — a function that,
// like the executable "@ file, produces the result.
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

// PromptGet is prompts/get: the template file, rendered (§2).
func (s *Store) PromptGet(ctx context.Context, name string, data map[string]string) (string, error) {
	b, err := s.Read(ctx, DirPrompts+"/"+name+".tmpl")
	if err != nil {
		return "", err
	}
	out := string(b)
	for k, v := range data {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out, nil
}

// PromptList is prompts/list: the names of /prompts/*.tmpl files (§2).
func (s *Store) PromptList(ctx context.Context) ([]string, error) {
	ps, err := s.drv.List(ctx, DirPrompts)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range ps {
		if strings.HasSuffix(p, ".tmpl") {
			out = append(out, promptName(p))
		}
	}
	sort.Strings(out)
	return out, nil
}
