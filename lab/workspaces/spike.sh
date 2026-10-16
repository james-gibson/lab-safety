#!/bin/bash
#
# spike.sh — step 1 of the workspace-VFS prototype (lab-safety#9).
#
# The priming scratch script from the plan (register §2): put the store's
# existence transition onto the lab's *real* instruments — a smoke-alarm target
# and an adhd light — before the prototype's own machinery exists. This is
# scaffolding, not a deliverable; the lights and alarm tests outlive it.
#
# It stands in the store's MCP surface with a trivial responder that reads the
# session-42 store from disk, so "the store exists" (plane -42i) is what the
# surface reports as healthy.
#
# Observed (2026-10-16): dark -> green (store appears) then green -> red (store
# ceases). Yellow is NOT reachable on this path — see the finding at the end and
# register §2.
#
# Usage: ./spike.sh
#
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS="${KB_SCRIPTS:-$HOME/projects/kb/scripts}"
ALARM_SH="$SCRIPTS/smoke-alarm/spin.sh"
ADHD_SH="$SCRIPTS/adhd/spin.sh"
NAME="${NAME:-ws42}"
ADHD_IDLE="${ADHD_IDLE:-http://127.0.0.1:9090/mcp}"

[ -x "$ALARM_SH" ] || { echo "smoke-alarm spin.sh not found at $ALARM_SH" >&2; exit 1; }
[ -x "$ADHD_SH" ] || { echo "adhd spin.sh not found at $ADHD_SH" >&2; exit 1; }

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ws42.XXXXXX")"
STORE="$TMP/42"
SURFLOG="$TMP/surface.log"
PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')"
PY=""

cleanup() {
	"$ADHD_SH" down "${NAME}dash" >/dev/null 2>&1 || true
	"$ALARM_SH" down "$NAME" >/dev/null 2>&1 || true
	[ -n "$PY" ] && kill "$PY" 2>/dev/null || true
	rm -rf "$TMP"
}
trap cleanup EXIT

mcp() { curl -s --max-time 4 -XPOST "http://127.0.0.1:$1/mcp" -H 'Content-Type: application/json' -d "$2"; }
lights_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.status"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["lights"])'; }
targets_of() { curl -s --max-time 4 "http://127.0.0.1:$1/status" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(t["id"]+"="+t["state"] for t in d.get("targets",[])))'; }
lights_list_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(l["name"]+"="+l["status"] for l in d["result"]["lights"]))'; }

echo "## phase 0 — dark: no store is monitored"
echo "   reachable vocabulary: $(lights_of 9090)  (idle adhd; dark = no light)"

echo
echo "## phase 1 — the store exists (session 42)"
mkdir -p "$STORE"/{.a2a,.state,messages,context/tools,context/resources,context/prompts}
printf '{"session":"42","state":"SUBMITTED"}\n' > "$STORE/.a2a/task.json"
printf 'SUBMITTED\n' > "$STORE/.state/current"
echo "   created: $STORE"

# The stand-in surface: MCP-ish JSON-RPC, healthy iff the store root exists.
python3 - "$STORE" "$PORT" <<'PY' >>"$SURFLOG" 2>&1 &
import http.server, socketserver, json, os, sys
store, port = sys.argv[1], int(sys.argv[2])
class H(http.server.BaseHTTPRequestHandler):
    def _send(self, code, obj):
        d = json.dumps(obj).encode()
        self.send_response(code); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(d))); self.end_headers(); self.wfile.write(d)
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0)); b = self.rfile.read(n)
        if not os.path.isdir(store):
            self._send(503, {"error": "no store"}); return
        try: req = json.loads(b)
        except Exception: req = {}
        m = req.get("method", ""); i = req.get("id", 1)
        if m == "initialize":
            out = {"jsonrpc": "2.0", "id": i, "result": {"protocolVersion": "2024-11-05",
                   "capabilities": {"tools": {}, "resources": {}},
                   "serverInfo": {"name": "workspace-42", "version": "0"}}}
        elif m == "tools/list":
            out = {"jsonrpc": "2.0", "id": i, "result": {"tools":
                   [{"name": "echo", "description": "stand-in", "inputSchema": {"type": "object"}}]}}
        else:
            out = {"jsonrpc": "2.0", "id": i, "result": {}}
        self._send(200, out)
    def do_GET(self):
        self._send(200 if os.path.isdir(store) else 503, {"ok": os.path.isdir(store)})
    def log_message(self, *a): pass
socketserver.TCPServer.allow_reuse_address = True
socketserver.TCPServer(("127.0.0.1", port), H).serve_forever()
PY
PY=$!
disown "$PY" 2>/dev/null || true
sleep 0.8

echo
echo "## phase 2 — the surface is monitored; the light appears"
"$ALARM_SH" up "$NAME" --target "http://127.0.0.1:$PORT/mcp" >/dev/null
APORT="$(cat "$SCRIPTS/smoke-alarm/instances/$NAME/port")"
"$ADHD_SH" up "${NAME}dash" --smoke-alarm "http://127.0.0.1:$APORT" >/dev/null
DPORT="$(cat "$SCRIPTS/adhd/instances/${NAME}dash/port")"
sleep 9

echo "   surface:             http://127.0.0.1:$PORT/mcp  (healthy iff the store exists)"
echo "   alarm target state:  $(targets_of "$APORT")"
echo "   adhd lights:         $(lights_list_of "$DPORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

# phase 3 — the transition back: the store stops existing.
echo
echo "## phase 3 — the store ceases to exist"
rm -rf "$STORE"
sleep 23
echo "   alarm target state:  $(targets_of "$APORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

echo
echo "## the transition, as the instrument recorded it"
LOG="$HOME/Library/Logs/kb-adhd-${NAME}dash.jsonl"
grep -h '"type":"light-update"' "$LOG" 2>/dev/null | python3 -c 'import sys,json
for l in sys.stdin:
    d=json.loads(l); print("   "+(d.get("old_status") or "?")+" -> "+(d.get("new_status") or "?")+"   (target "+str(d.get("target_id"))+", via "+str(d.get("via"))+")")'

echo
echo "## finding"
echo "   The reachable light vocabulary here is {green, red}; dark is the no-light default."
echo "   Yellow (plane -42i) has no producer on the headless smoke-alarm path — it needs a"
echo "   dissent signal. So the store reaching -42i reads GREEN, not yellow. See register §2."
