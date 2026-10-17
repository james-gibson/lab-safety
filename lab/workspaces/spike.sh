#!/bin/bash
#
# spike.sh — the workspace-VFS prototype on the lab's *real* instruments.
#
# rung 1 (register §1): the store exists, on a real host, and the same spec holds
#   on tmpdir and sqlite.
# rung 2: the *surface* is a real MCP server in its own process, seeded from the
#   spec, and the lab's own smoke-alarm probes it over the wire (a `ProtocolMCP`
#   target). The verdict is the instrument's, not this package's suite — "go test"
#   is rung 0, self-report.
#
# Step 1 primed this loop with a python stand-in that answered "healthy iff the
# store directory exists". The stand-in is **retired**: `cmd/workspace-mcp` now
# serves the store's own MCP surface. Nothing here is MCP-ish; it is MCP.
#
# Observed (2026-10-16): dark -> green (the session exists) -> red (the session
# goes away *while the server keeps running*). Yellow is still NOT reachable —
# see the finding at the end and register §2.
#
# Usage: ./spike.sh                     # the tmpdir host (the fast one)
#        DRIVER=sqlite ./spike.sh       # the sqlite host (the mirror)
#
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS="${KB_SCRIPTS:-$HOME/projects/kb/scripts}"
ALARM_SH="$SCRIPTS/smoke-alarm/spin.sh"
ADHD_SH="$SCRIPTS/adhd/spin.sh"
NAME="${NAME:-ws42}"
DRIVER="${DRIVER:-tmpdir}"
GATE="${GATE:-$HERE/../../sheets/workspaces/minimal.feature}"
ADHD_IDLE="${ADHD_IDLE:-http://127.0.0.1:9090/mcp}"

[ -x "$ALARM_SH" ] || { echo "smoke-alarm spin.sh not found at $ALARM_SH" >&2; exit 1; }
[ -x "$ADHD_SH" ] || { echo "adhd spin.sh not found at $ADHD_SH" >&2; exit 1; }
[ -f "$GATE" ] || { echo "the gate not found at $GATE" >&2; exit 1; }

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ws42.XXXXXX")"
SRVLOG="$TMP/surface.log"
LOG="$HOME/Library/Logs/kb-adhd-${NAME}dash.jsonl"
SRV=""
if [ "$DRIVER" = sqlite ]; then ROOT="$TMP/42.wsdb"; else ROOT="$TMP/42"; fi

# clear any instance left behind by an earlier run
"$ADHD_SH" down "${NAME}dash" >/dev/null 2>&1 || true
"$ALARM_SH" down "$NAME" >/dev/null 2>&1 || true
: >"$LOG" 2>/dev/null || true

cleanup() {
	"$ADHD_SH" down "${NAME}dash" >/dev/null 2>&1 || true
	"$ALARM_SH" down "$NAME" >/dev/null 2>&1 || true
	[ -n "$SRV" ] && kill "$SRV" 2>/dev/null || true
	rm -rf "$TMP"
}
trap cleanup EXIT

mcp() { curl -s --max-time 4 -XPOST "http://127.0.0.1:$1/mcp" -H 'Content-Type: application/json' -d "$2"; }
strict() { mcp "$1" "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":{}}"; }
field() { python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("?"); raise SystemExit
e=d.get("error"); r=d.get("result")
print(e.get("message") if e else json.dumps(r)[:150])'; }
targets_of() { curl -s --max-time 4 "http://127.0.0.1:$1/status" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(t["id"]+"="+t["state"] for t in d.get("targets",[])))'; }
lights_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.status"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["lights"])'; }
lights_list_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(l["name"]+"="+l["status"] for l in d["result"]["lights"]))'; }

echo "## build — the surface is a real MCP server, not a stand-in"
( cd "$HERE" && go build -o "$TMP/workspace-mcp" ./cmd/workspace-mcp ) || exit 1
echo "   built: workspace-mcp   (driver $DRIVER, session 42)"

echo
echo "## phase 0 — dark: no session, no surface, no light"
echo "   reachable vocabulary: $(lights_of 9090)  (idle adhd; dark = no light)"

echo
echo "## phase 1 — the session exists, and its surface is up (session 42)"
"$TMP/workspace-mcp" -driver "$DRIVER" -root "$ROOT" -session 42 \
	-spec "$GATE" -key seed-42 -addr 127.0.0.1:0 >"$SRVLOG" 2>&1 &
SRV=$!
disown "$SRV" 2>/dev/null || true

PORT=""
for _ in $(seq 1 60); do
	PORT="$(sed -n 's#.*listening http://127.0.0.1:\([0-9]*\)/mcp#\1#p' "$SRVLOG" | head -1)"
	[ -n "$PORT" ] && break
	sleep 0.25
done
[ -n "$PORT" ] || { echo "   the surface did not come up:"; cat "$SRVLOG"; exit 1; }

echo "   $(head -1 "$SRVLOG")"
echo "   surface:     http://127.0.0.1:$PORT/mcp   (pid $SRV)"
echo "   initialize:  $(strict "$PORT" initialize | field)"
echo "   tools/list:  $(strict "$PORT" tools/list | field)"
echo "   -- the handshake the alarm will perform, answered by the store --"

echo
echo "## phase 2 — the surface is monitored; the light appears"
"$ALARM_SH" up "$NAME" --target "http://127.0.0.1:$PORT/mcp" >/dev/null
APORT="$(cat "$SCRIPTS/smoke-alarm/instances/$NAME/port")"
"$ADHD_SH" up "${NAME}dash" --smoke-alarm "http://127.0.0.1:$APORT" >/dev/null
DPORT="$(cat "$SCRIPTS/adhd/instances/${NAME}dash/port")"
sleep 12

echo "   alarm target state:  $(targets_of "$APORT")"
echo "   adhd lights:         $(lights_list_of "$DPORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

echo
echo "## phase 3 — the session goes away (the server keeps running)"
case "$DRIVER" in
	sqlite) sqlite3 "$ROOT" 'DELETE FROM files' ;;
	*)      rm -rf "$ROOT" ;;
esac
sleep 25

if kill -0 "$SRV" 2>/dev/null; then SERVER_STATE=alive; else SERVER_STATE=GONE; fi
echo "   the server is still $SERVER_STATE (pid $SRV)"
echo "   initialize now:      $(strict "$PORT" initialize | field)"
echo "   alarm target state:  $(targets_of "$APORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

echo
echo "## the transition, as the instrument recorded it"
grep -h '"type":"light-update"' "$LOG" 2>/dev/null | python3 -c 'import sys,json
for l in sys.stdin:
    d=json.loads(l); print("   "+(d.get("old_status") or "?")+" -> "+(d.get("new_status") or "?")+"   (target "+str(d.get("target_id"))+", via "+str(d.get("via"))+")")'

echo
echo "## finding (rung 2)"
echo "   The reachable light vocabulary on a smoke-alarm target is {green, red}; dark is the"
echo "   no-light default. The surface now reports its *session*, so the light tracks the"
echo "   session and not the process — but yellow (plane -42i) still has no producer."
echo "   adhd *does* map degraded -> yellow (internal/smokelink/watcher.go, mapHealthState);"
echo "   it is smoke-alarm's prober that never yields degraded: every failure is"
echo "   unhealthy/outage -> red, and a capability mismatch is not checked at all. To see"
echo "   -42i the alarm must be able to say \"answering, but not what was declared\" —"
echo "   james-gibson/smoke-alarm#34."
