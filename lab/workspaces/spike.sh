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
# rung 2b: the session is a **node**, not an anonymous port. It registers by name
#   in the lab's own isotope registry, and the monitor resolves it *by name* —
#   `--target-session` — so no caller ever carries the session's address. That is
#   the lab's own model (TRUST-FLOWS §2: A = session, B = surface, C = authority;
#   §5: the wiring is decided out of band), and it is why this script no longer
#   knows the session's port, nor the monitor's.
#
# Observed (2026-10-17, with smoke-alarm#34 landed): the whole plane —
#
#   dark ──▶ green ──▶ yellow ──▶ red
#   (no session)  (it exists)  (it lost what the spec declares)  (it goes away)
#
# The middle rung is *"answering, but not as declared"*: the surface stops
# advertising a capability its session can no longer serve, and the target — which
# declared that capability must be there — reads degraded rather than healthy or
# down. Point SMOKE_ALARM_SRC at a smoke-alarm checkout to build the fixed monitor.
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
SESSION="${SESSION:-42}"
ISOTOPE="workspace-$SESSION"
GATE="${GATE:-$HERE/../../sheets/workspaces/minimal.feature}"
ADHD_IDLE="${ADHD_IDLE:-http://127.0.0.1:9090/mcp}"

# The three nodes this run needs, all named by the same prefix.
REG="${NAME}reg"    # the registry: a running alarm that holds names -> endpoints
MON="${NAME}mon"    # the monitor: resolves the session by name, then probes it
DASH="${NAME}dash"  # the dashboard: the light

[ -x "$ALARM_SH" ] || { echo "smoke-alarm spin.sh not found at $ALARM_SH" >&2; exit 1; }
[ -x "$ADHD_SH" ] || { echo "adhd spin.sh not found at $ADHD_SH" >&2; exit 1; }
[ -f "$GATE" ] || { echo "the gate not found at $GATE" >&2; exit 1; }

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ws42.XXXXXX")"
SRVLOG="$TMP/surface.log"
LOG="$HOME/Library/Logs/kb-adhd-${DASH}.jsonl"
SRV=""
if [ "$DRIVER" = sqlite ]; then ROOT="$TMP/$SESSION.wsdb"; else ROOT="$TMP/$SESSION"; fi

cleanup() {
	"$ADHD_SH" down "$DASH" >/dev/null 2>&1 || true
	"$ALARM_SH" down "$MON" >/dev/null 2>&1 || true
	"$ALARM_SH" down "$REG" >/dev/null 2>&1 || true
	[ -n "$SRV" ] && kill "$SRV" 2>/dev/null || true
	rm -rf "$TMP"
}
# clear anything an earlier run left behind
"$ADHD_SH" down "$DASH" >/dev/null 2>&1 || true
"$ALARM_SH" down "$MON" >/dev/null 2>&1 || true
"$ALARM_SH" down "$REG" >/dev/null 2>&1 || true
: >"$LOG" 2>/dev/null || true
trap cleanup EXIT

mcp() { curl -s --max-time 4 -XPOST "http://127.0.0.1:$1/mcp" -H 'Content-Type: application/json' -d "$2"; }
strict() { mcp "$1" "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":{}}"; }
field() { python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("?"); raise SystemExit
e=d.get("error"); r=d.get("result")
print(e.get("message") if e else json.dumps(r)[:150])'; }
caps() { python3 -c 'import sys,json
d=json.load(sys.stdin)
c=(d.get("result") or {}).get("capabilities", {})
print(",".join(sorted(c)) if c else "(none)")'; }
targets_of() { curl -s --max-time 4 "http://127.0.0.1:$1/status" | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(t["id"]+"="+t["state"] for t in d.get("targets",[])))'; }
registry_of() { curl -s --max-time 4 "http://127.0.0.1:$1/isotope/list" | python3 -c 'import sys,json;recs=json.load(sys.stdin);print("; ".join("%s=%s rung %s" % (r["name"], r["endpoint"], r.get("trust_rung")) for r in recs) or "(empty)")'; }
lights_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.status"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["lights"])'; }
lights_list_of() { mcp "$1" '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' | python3 -c 'import sys,json;d=json.load(sys.stdin);print("; ".join(l["name"]+"="+l["status"] for l in d["result"]["lights"]))'; }

echo "## build — the surface is a real MCP server, not a stand-in"
( cd "$HERE" && go build -o "$TMP/workspace-mcp" ./cmd/workspace-mcp ) || exit 1
echo "   built: workspace-mcp   (driver $DRIVER, session $SESSION)"
if [ -n "${SMOKE_ALARM_SRC:-}" ]; then
	( cd "$SMOKE_ALARM_SRC" && go build -o "$TMP/ocd-smoke-alarm" ./cmd/ocd-smoke-alarm ) || exit 1
	export ALARM_BIN="$TMP/ocd-smoke-alarm"
	echo "   built: the monitor from $SMOKE_ALARM_SRC"
fi

echo
echo "## phase 0 — dark: no session, no surface, no light"
echo "   reachable vocabulary: $(lights_of 9090)  (idle adhd; dark = no light)"

echo
echo "## phase 1 — the registry, then the session registers itself by name"
"$ALARM_SH" up "$REG" --empty >/dev/null
RPORT="$(cat "$SCRIPTS/smoke-alarm/instances/$REG/port")"
echo "   registry:    a running alarm at 127.0.0.1:$RPORT  (names -> endpoints)"
echo "   registry now: $(registry_of "$RPORT")"

"$TMP/workspace-mcp" -driver "$DRIVER" -root "$ROOT" -session "$SESSION" \
	-spec "$GATE" -key seed-42 -smoke-alarm "http://127.0.0.1:$RPORT" \
	-addr 127.0.0.1:0 >"$SRVLOG" 2>&1 &
SRV=$!
disown "$SRV" 2>/dev/null || true

SURF=""
for _ in $(seq 1 60); do
	SURF="$(sed -n 's/.*registered name=[^ ]* endpoint=\([0-9.]*:[0-9]*\).*/\1/p' "$SRVLOG" | head -1)"
	[ -n "$SURF" ] && break
	sleep 0.25
done
[ -n "$SURF" ] || { echo "   the session did not come up:"; cat "$SRVLOG"; exit 1; }
SPORT="${SURF##*:}"

echo "   $(grep -m1 '^listening' "$SRVLOG")"
echo "   $(grep -m1 '^registered' "$SRVLOG")"
echo "   registry now: $(registry_of "$RPORT")"
echo "   initialize:  $(strict "$SPORT" initialize | field)"
echo "   -- the session announced itself; the registry answers for it --"

echo
echo "## phase 2 — the monitor resolves the session BY NAME; the light appears"
echo "   mon:  --target-session $ISOTOPE --registry http://127.0.0.1:$RPORT"
echo "         (the session's address is never passed to it)"
"$ALARM_SH" up "$MON" --target-session "$ISOTOPE" --registry "http://127.0.0.1:$RPORT" >/dev/null
"$ADHD_SH" up "$DASH" --smoke-alarm-instance "$MON" >/dev/null
MPORT="$(cat "$SCRIPTS/smoke-alarm/instances/$MON/port")"
DPORT="$(cat "$SCRIPTS/adhd/instances/$DASH/port")"
sleep 12

echo "   alarm target state:  $(targets_of "$MPORT")"
echo "   adhd lights:         $(lights_list_of "$DPORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

echo
echo "## phase 3 — the session loses what the spec declares (the server keeps running)"
case "$DRIVER" in
	sqlite) sqlite3 "$ROOT" "DELETE FROM files WHERE path = '/tools/echo@'" ;;
	*)      rm -f "$ROOT/tools/echo@" ;;
esac
sleep 25

echo "   the declared tool is gone, so the surface stops promising 'tools':"
echo "   capabilities now:    $(strict "$SPORT" initialize | caps)"
echo "   initialize now:      $(strict "$SPORT" initialize | field)"
echo "   alarm target state:  $(targets_of "$MPORT")"
echo "   adhd lights:         $(lights_list_of "$DPORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"

echo
echo "## phase 4 — the session goes away (the server keeps running)"
case "$DRIVER" in
	sqlite) sqlite3 "$ROOT" 'DELETE FROM files' ;;
	*)      rm -rf "$ROOT" ;;
esac
sleep 25

if kill -0 "$SRV" 2>/dev/null; then SERVER_STATE=alive; else SERVER_STATE=GONE; fi
echo "   the server is still $SERVER_STATE (pid $SRV)"
echo "   initialize now:      $(strict "$SPORT" initialize | field)"
echo "   alarm target state:  $(targets_of "$MPORT")"
echo "   adhd summary:        $(lights_of "$DPORT")"
echo "   registry now:        $(registry_of "$RPORT")   <- still listed: a name is not a heartbeat"

echo
echo "## the transition, as the instrument recorded it"
grep -h '"type":"light-update"' "$LOG" 2>/dev/null | python3 -c 'import sys,json
for l in sys.stdin:
    d=json.loads(l); print("   "+(d.get("old_status") or "?")+" -> "+(d.get("new_status") or "?")+"   (target "+str(d.get("target_id"))+", via "+str(d.get("via"))+")")'

echo
echo "## the finding, closed (rung 2 / smoke-alarm#34)"
echo "   yellow (plane -42i) now has a producer: a target that *declared* what the"
echo "   surface must advertise, against a surface that answers while no longer"
echo "   advertising it. That is the middle rung — \"exists, but not as declared\" —"
echo "   and it needed both halves:"
echo "     * smoke-alarm  — the prober classifies a partial handshake and an"
echo "       unadvertised min_capability as degraded, and the engine no longer"
echo "       rewrites a *degraded* result into a regression or an outage;"
echo "     * the surface  — advertises what the session can actually serve."
echo "   adhd needed no change: mapHealthState already read degraded -> yellow."
echo
echo "   What red still means, and should: the session not answering at all. A"
echo "   departure is yellow; a failure is red."
