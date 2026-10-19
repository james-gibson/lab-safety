# workspace-agent — the virtual tree in an editor, before the front door

A **debugging** ACP v1 agent over one session's workspace VFS. There is no model
here: a prompt *is* a command, executed against the session's store and streamed
back as an ACP tool call.

It exists so the tree can be driven from an editor (Zed over stdio) while the
tree is being built — and so the real ACP wire is exercised without standing up
the safety apparatus. It is **not** the lab's ACP front door and it does not
claim conformance. See `lab/SLA.md` §4: the front door, when it exists, follows
the published handshake; this shim exists to be thrown away.

## Run

```sh
go build -o /tmp/workspace-agent ./cmd/workspace-agent

/tmp/workspace-agent -driver tmpdir -root /tmp/ws42/42
/tmp/workspace-agent -driver sqlite -root /tmp/ws42/42.wsdb \
  -spec ../../sheets/workspaces/minimal.feature -key seed-42
```

Stdout carries the ACP protocol; logs go to stderr. With `-spec` the store is
seeded from the Gherkin and every declared tool gets the echo stand-in, as in
`cmd/workspace-mcp`. Without it, a store that is not present is initialised.

## In an editor

Zed launches the binary as an agent over stdio. Add an `agent_servers` entry
whose `command` is the built binary and whose `args` are the flags above:

```json
{
  "agent_servers": {
    "workspace-42": {
      "command": "/tmp/workspace-agent",
      "args": ["-driver", "tmpdir", "-root", "/tmp/ws42/42"]
    }
  }
}
```

The client's `cwd` is **ignored** (and logged). The session's `/` is the
workspace namespace, with no parent above it
(`sheets/workspaces/filesystem.feature`, Rule 1) — not the directory the editor
opened.

## Verbs

| verb | does |
| --- | --- |
| `ls [path]` | list paths under a prefix (default `/`) |
| `tree` | list the whole session tree |
| `cat <path>` | read a file |
| `write <path> <text>` | write a resource, tool, prompt, or plain file |
| `mkdir <path>` / `rm <path>` | create / remove a path |
| `tools` | list `/tools/*@` |
| `resources` / `prompts` | list the session's resources / prompts |
| `call <tool> [json]` | `tools/call` through the in-process MCP surface |
| `state` / `session` | `/state/current`; session id, driver, presence |
| `help` | the list above |

`write` dispatches to the store's own writers for `/tools/`, `/resources/` and
`/prompts/`, so dropping `/tools/x@` is the dynamic-command test (A11) from the
editor. `call` goes through `Protocol.Handle` — the console sees what a caller
sees, including `-32001 no session here: 42` when the store is absent.

## What it deliberately does not do

- **No model, no planning, no permission flow.** The prompt is the command.
- **No client `fs/*` or `terminal/*` calls.** Those are optional v1 agent→client
  interfaces; none is implemented, so the SDK answers `method not found` rather
  than advertising a capability the agent does not have. ACP v2 removes them
  anyway (`rfds/v2/client-filesystem-terminal-capabilities`).
- **v1 only.** v2 is a draft; `router` can add it later without changing this
  shim.
- **One store per process.** Multiple ACP sessions share the one workspace
  session (`42` by default); it is a console, not a session manager.
- **A plain-file `write` is untraced.** `/tools`, `/resources` and `/prompts`
  go through the store writers and are noted; anything else goes to the driver
  directly and does not appear on the recorder. That is the known cost of the
  cheat, not a claim about the write path.

## Dependency

`github.com/ironpark/acp-go v0.2.0` (MIT, unofficial, pins ACP v1 `1.25.0` and
v2 draft `2.0.0-alpha.8`). Pinned deliberately: pre-1.0 SDK, draft protocol.
The alarm's prober must **not** import it — a verifier that shares the
implementation agrees with it by construction.

## Test

```sh
go test ./...
```

`agent_test.go` drives the agent over `acp1.Pipe` — the same surface an editor
does, without spawning a child — and asserts the commands, the store-writer
dispatch, and that `/../etc/passwd` is refused structurally.
