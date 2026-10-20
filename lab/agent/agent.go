// Package agent is a *debugging* ACP agent over a session's workspace VFS.
//
// It is not the lab's front door, and it is deliberately not a model: there is
// no LLM here. A prompt is a command, executed against the session's store and
// streamed back as an ACP tool call. The point is to put the virtual tree in an
// editor over stdio while the tree itself is being built, and to exercise the
// real ACP wire without standing up the safety apparatus.
//
// Deliberate non-goals: no model, no permissions, no client fs/terminal calls,
// no v2. This is ACP v1 and only the four methods every agent must handle. The
// SDK routes optional methods by advertised capability; nothing optional is
// implemented, so a client is told "method not found" rather than being served
// a capability this agent does not have.
//
// A command writes through the store's own writers (`AddTool`, `AddResource`,
// `AddPrompt`), so those writes are noted on the store's recorder like any
// other. A plain file write goes to the driver directly and is *not* noted —
// the debug shim is outside the write path there, and that is a known cost
// rather than a claim.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"

	acp1 "github.com/ironpark/acp-go/acp1"

	"github.com/james-gibson/lab-safety/workspaces"
)

// Agent implements acp1.Agent over one workspace store.
type Agent struct {
	client acp1.Client
	store  *workspaces.Store
	driver string
	proto  *workspaces.Protocol
	seq    atomic.Uint64
}

// New returns an agent that answers one ACP connection against s.
func New(client acp1.Client, s *workspaces.Store, driver string) *Agent {
	return &Agent{
		client: client,
		store:  s,
		driver: driver,
		proto:  workspaces.NewProtocol(s),
	}
}

// RegisterTool binds a stand-in implementation for a declared tool, so
// `call <name>` can exercise `tools/call` in-process.
func (a *Agent) RegisterTool(name string, fn func([]byte) []byte) {
	a.proto.RegisterTool(name, fn)
}

// Initialize negotiates the protocol version and advertises only what is
// implemented: the four required methods and no optional capabilities.
func (a *Agent) Initialize(context.Context, *acp1.InitializeRequest) (*acp1.InitializeResponse, error) {
	return &acp1.InitializeResponse{
		ProtocolVersion:   acp1.ProtocolVersion,
		AgentCapabilities: acp1.CapabilitiesOf(a),
		AgentInfo: &acp1.Implementation{
			Name:    "workspace-agent",
			Version: "0.0.1-debug",
		},
	}, nil
}

// NewSession creates an ACP session. The client's cwd is *not* the workspace
// root: the session's "/" is the store's namespace, and its parent does not
// exist (sheets/workspaces/filesystem.feature). The cwd is recorded on stderr
// so the mismatch is visible rather than silently reconciled.
func (a *Agent) NewSession(_ context.Context, params *acp1.NewSessionRequest) (*acp1.NewSessionResponse, error) {
	if params != nil && params.Cwd != "" {
		log.Printf("workspace-agent: client cwd %q ignored; the session root is the workspace %q", params.Cwd, a.store.Session())
	}
	return &acp1.NewSessionResponse{SessionID: acp1.GenerateSessionID()}, nil
}

// Prompt runs one command as one tool call. An error is reported as a failed
// tool call and a visible message, not as a failed turn: a debug console should
// show the refusal.
func (a *Agent) Prompt(ctx context.Context, params *acp1.PromptRequest) (*acp1.PromptResponse, error) {
	stream := acp1.NewSessionStream(a.client, params.SessionID)

	text := strings.TrimSpace(acp1.JoinTexts(params.Prompt))
	cmd := parse(text)
	id := acp1.ToolCallID("ws-" + strconv.FormatUint(a.seq.Add(1), 10))

	kind := acp1.ToolKindRead
	if cmd.mutates() {
		kind = acp1.ToolKindExecute
	}
	if err := stream.StartToolCall(ctx, id, cmd.title(), kind); err != nil {
		return nil, err
	}

	out, err := a.exec(ctx, cmd)
	if err != nil {
		_ = stream.FailToolCall(ctx, id, acp1.WithToolContent(acp1.ToolText(err.Error())))
		if sendErr := stream.SendText(ctx, "error: "+err.Error()); sendErr != nil {
			return nil, sendErr
		}
		return &acp1.PromptResponse{StopReason: acp1.StopReasonEndTurn}, nil
	}

	if err := stream.CompleteToolCall(ctx, id, acp1.WithToolContent(acp1.ToolText(out))); err != nil {
		return nil, err
	}
	if err := stream.SendText(ctx, out); err != nil {
		return nil, err
	}
	return &acp1.PromptResponse{StopReason: acp1.StopReasonEndTurn}, nil
}

// CancelSession has nothing to stop: every command is synchronous and short.
func (a *Agent) CancelSession(context.Context, *acp1.CancelNotification) error { return nil }

// ---------- the debug verbs ----------

type command struct {
	verb string
	arg  string
	rest string
	raw  string
}

func parse(text string) command {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(line)
	fields := strings.SplitN(line, " ", 3)

	c := command{raw: line}
	if len(fields) > 0 {
		c.verb = strings.ToLower(strings.TrimSpace(fields[0]))
	}
	if len(fields) > 1 {
		c.arg = strings.TrimSpace(fields[1])
	}
	if len(fields) > 2 {
		c.rest = fields[2]
	}
	return c
}

func (c command) mutates() bool {
	switch c.verb {
	case "write", "mkdir", "rm", "call":
		return true
	}
	return false
}

func (c command) title() string {
	if c.raw == "" {
		return "workspace: help"
	}
	return "workspace: " + c.raw
}

func (a *Agent) exec(ctx context.Context, c command) (string, error) {
	switch c.verb {
	case "", "help", "?":
		return helpText, nil

	case "ls", "list":
		p := virtualPath(c.arg)
		paths, err := a.store.Driver().List(ctx, p)
		if err != nil {
			return "", err
		}
		if len(paths) == 0 {
			return "(empty) " + p, nil
		}
		return strings.Join(paths, "\n"), nil

	case "tree":
		paths, err := a.store.Tree(ctx)
		if err != nil {
			return "", err
		}
		return strings.Join(paths, "\n"), nil

	case "cat", "read":
		if c.arg == "" {
			return "", fmt.Errorf("cat needs a path")
		}
		b, err := a.store.Read(ctx, virtualPath(c.arg))
		if err != nil {
			return "", err
		}
		return string(b), nil

	case "write":
		if c.arg == "" {
			return "", fmt.Errorf("write needs a path and text")
		}
		p := virtualPath(c.arg)
		body := []byte(c.rest)
		if err := a.write(ctx, p, body); err != nil {
			return "", err
		}
		return fmt.Sprintf("wrote %d bytes to %s", len(body), p), nil

	case "mkdir":
		if c.arg == "" {
			return "", fmt.Errorf("mkdir needs a path")
		}
		p := virtualPath(c.arg)
		if err := a.store.Driver().Mkdir(ctx, p); err != nil {
			return "", err
		}
		return "mkdir " + p, nil

	case "rm":
		if c.arg == "" {
			return "", fmt.Errorf("rm needs a path")
		}
		p := virtualPath(c.arg)
		if err := a.store.Driver().Delete(ctx, p); err != nil {
			return "", err
		}
		return "removed " + p, nil

	case "tools":
		names, err := a.store.ToolsList(ctx)
		if err != nil {
			return "", err
		}
		return listOrEmpty(names, "tools"), nil

	case "resources":
		uris, err := a.store.ResourcesList(ctx)
		if err != nil {
			return "", err
		}
		return listOrEmpty(uris, "resources"), nil

	case "prompts":
		names, err := a.store.PromptList(ctx)
		if err != nil {
			return "", err
		}
		return listOrEmpty(names, "prompts"), nil

	case "call":
		if c.arg == "" {
			return "", fmt.Errorf("call needs a tool name")
		}
		return a.callTool(ctx, c.arg, c.rest)

	case "state":
		s, err := a.store.State(ctx)
		if err != nil {
			return "", err
		}
		return s, nil

	case "session":
		present, err := a.store.Present(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("session=%s driver=%s present=%t", a.store.Session(), a.driver, present), nil

	default:
		return "", fmt.Errorf("unknown verb %q\n\n%s", c.verb, helpText)
	}
}

// write dispatches to the store's own writers where a path has one, so the
// MCP mapping (a tool is a file, a resource is a file, a prompt is a file) is
// what changes, not a second convention invented for the console.
func (a *Agent) write(ctx context.Context, p string, body []byte) error {
	switch {
	case strings.HasPrefix(p, workspaces.DirTools+"/"):
		name := strings.TrimSuffix(strings.TrimPrefix(p, workspaces.DirTools+"/"), "@")
		return a.store.AddTool(ctx, name, body)
	case strings.HasPrefix(p, workspaces.DirResources+"/"):
		return a.store.AddResource(ctx, strings.TrimPrefix(p, workspaces.DirResources+"/"), body)
	case strings.HasPrefix(p, workspaces.DirPrompts+"/"):
		return a.store.AddPrompt(ctx, strings.TrimPrefix(p, workspaces.DirPrompts+"/"), body)
	default:
		return a.store.Driver().Put(ctx, p, body)
	}
}

// callTool goes through the MCP surface rather than calling the function: the
// console should see what a caller sees, including the "no session here"
// refusal when the store is absent.
func (a *Agent) callTool(ctx context.Context, name, args string) (string, error) {
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}
	if !json.Valid([]byte(args)) {
		return "", fmt.Errorf("call %s: arguments are not JSON: %s", name, args)
	}
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": json.RawMessage(args),
		},
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(a.proto.Handle(ctx, req), &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("%d: %s", resp.Error.Code, resp.Error.Message)
	}
	parts := make([]string, 0, len(resp.Result.Content))
	for _, c := range resp.Result.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func virtualPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "~" {
		return workspaces.Root
	}
	p = strings.TrimPrefix(p, "~")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func listOrEmpty(items []string, what string) string {
	if len(items) == 0 {
		return "(no " + what + ")"
	}
	return strings.Join(items, "\n")
}

const helpText = `workspace-agent — the session tree as commands (ACP v1, debug only)

  ls [path]            list paths under a prefix (default /)
  tree                 list the whole session tree
  cat <path>           read a file
  write <path> <text>  write a resource, tool, prompt, or plain file
  mkdir <path>         create a directory
  rm <path>            remove a path
  tools                list /tools/*@
  resources            list the session's resources
  prompts              list the session's prompts
  call <tool> [json]   tools/call through the MCP surface
  state                /state/current
  session              session id, driver, and presence
  help                 this

The prompt is the command. There is no model and no permission flow.`
