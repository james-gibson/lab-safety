#!/bin/bash
#
# spike.sh — the WAN front on the lab's real instruments.
#
# It drives the loop the Gherkin declares, with the verdict coming from the
# lab's own monitor rather than this package's tests:
#
#   dark (no light) ──▶ yellow (defensive: answers, promises no forwarding)
#                   ──▶ green  (agreed traffic crossed the signed border)
#                   ──▶ red    (the process is gone)
#
# smoke-alarm declares `ingress/forward` as min_capability; the front advertises
# that family only after a real forward. adhd renders the alarm's degraded /
# healthy / unhealthy states as yellow / green / red. KB_SCRIPTS points at the
# vault's spin harnesses; nothing here invents a monitor.
#
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
SCRIPTS="${KB_SCRIPTS:-$HOME/projects/kb/scripts}"
ALARM_SH="$SCRIPTS/smoke-alarm/spin.sh"
ADHD_SH="$SCRIPTS/adhd/spin.sh"
NAME="${NAME:-wan}"
MON="${NAME}mon"
DASH="${NAME}dash"

[ -x "$ALARM_SH" ] || { echo "smoke-alarm spin.sh not found at $ALARM_SH (set KB_SCRIPTS)" >&2; exit 1; }
[ -x "$ADHD_SH" ] || { echo "adhd spin.sh not found at $ADHD_SH (set KB_SCRIPTS)" >&2; exit 1; }

TMP="$(mktemp -d "${TMPDIR:-/tmp}/wan-ingress.XXXXXX")"
LOG="$HOME/Library/Logs/kb-adhd-${DASH}.jsonl"
ING=""
BORDER=""
PROXY=""
cleanup() {
	"$ADHD_SH" down "$DASH" >/dev/null 2>&1 || true
	"$ALARM_SH" down "$MON" >/dev/null 2>&1 || true
	if [ -n "$ING" ]; then kill "$ING" 2>/dev/null || true; wait "$ING" 2>/dev/null || true; fi
	if [ -n "$BORDER" ]; then kill "$BORDER" 2>/dev/null || true; wait "$BORDER" 2>/dev/null || true; fi
	if [ -n "$PROXY" ]; then kill "$PROXY" 2>/dev/null || true; wait "$PROXY" 2>/dev/null || true; fi
	rm -rf "$TMP"
}
"$ADHD_SH" down "$DASH" >/dev/null 2>&1 || true
"$ALARM_SH" down "$MON" >/dev/null 2>&1 || true
: >"$LOG" 2>/dev/null || true
trap cleanup EXIT

free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }
IPORT="$(free_port)"
BPORT="$(free_port)"
PPORT="$(free_port)"
TOKEN="spike-token"
KEY="spike-border-key"

json() { python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("?"); raise SystemExit
print(json.dumps(d)[:200])'; }
targets() { curl -s --max-time 4 "http://127.0.0.1:$1/status" | python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("?"); raise SystemExit
print("; ".join(t["id"]+"="+t["state"] for t in d.get("targets",[])) or "(none)")'; }
lights() { curl -s --max-time 4 -XPOST "http://127.0.0.1:$1/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' | python3 -c 'import sys,json
d=json.load(sys.stdin)
print("; ".join(l["name"]+"="+l["status"] for l in d.get("result",{}).get("lights",[])) or "(dark)")'; }
registry() { curl -s --max-time 4 "http://127.0.0.1:$1/isotope/list" | python3 -c 'import sys,json
try: d=json.load(sys.stdin)
except Exception: print("?"); raise SystemExit
print("; ".join(r.get("name","?")+"="+r.get("endpoint","?") for r in d) or "(empty)")'; }
wait_for_text() { # <description> <pattern> <command...>
	local desc="$1" pattern="$2"; shift 2
	local out
	for _ in $(seq 1 40); do
		out="$("$@" 2>/dev/null || true)"
		if grep -q -- "$pattern" <<<"$out"; then
			printf '%s\n' "$out"
			return 0
		fi
		sleep 1
	done
	echo "FAILED waiting for $desc (last: $out)" >&2
	return 1
}

echo "## build — the front is the real binary"
( cd "$REPO" && go build -o "$TMP/wan-ingress" ./cmd/wan-ingress ) || exit 1
( cd "$REPO" && go build -o "$TMP/invite-proxy" ./sheets/ingress/proxy ) || exit 1
if [ -n "${SMOKE_ALARM_SRC:-}" ]; then
	( cd "$SMOKE_ALARM_SRC" && go build -o "$TMP/ocd-smoke-alarm" ./cmd/ocd-smoke-alarm ) || exit 1
	export ALARM_BIN="$TMP/ocd-smoke-alarm"
fi

echo "## phase 0 — the monitor and the light come up"
"$ALARM_SH" up "$MON" --empty >/dev/null || exit 1
MPORT="$(cat "$SCRIPTS/smoke-alarm/instances/$MON/port")"
echo "   registry: $(registry "$MPORT")"
echo "   lights:   (dark — adhd is not up yet)"

# The visitor's address is an invite, not a URL. This proxy stands in for the
# peer cluster's egress/alarms: it resolves the isotope name in the registry and
# forwards. Neither the monitor nor the forward test knows the ingress port.
"$TMP/invite-proxy" -addr="127.0.0.1:$PPORT" -registry="http://127.0.0.1:$MPORT" \
	-name="wan-ingress" -token="$TOKEN" >"$TMP/proxy.log" 2>&1 &
PROXY=$!
sleep 1
echo "   invite:   wan-ingress → registry → current binding (no URL held by the caller)"

# spin.sh's stock target is a strict MCP/tools target. Swap in the declaration
# this feature makes, then let smoke-alarm reload it over SIGHUP — its own
# config-reload path, with no change to the vault tooling.
CFG="$SCRIPTS/smoke-alarm/instances/$MON/config.yaml"
python3 - "$CFG" "$PPORT" <<'PY'
import sys
path, port = sys.argv[1], sys.argv[2]
text = open(path).read()
target = '''targets:
  - id: "wan-ingress"
    enabled: true
    protocol: "mcp"
    name: "WAN Ingress"
    endpoint: "http://127.0.0.1:%s/mcp"
    transport: "http"
    expected:
      healthy_status_codes: [200]
      min_capabilities:
        - "ingress/forward"
    auth:
      type: "none"
    check:
      interval: "2s"
      timeout: "2s"
      retries: 1
      handshake_profile: "base"
      required_methods:
        - "initialize"
        - "ingress/status"
''' % port
if 'targets: []' not in text:
    raise SystemExit('expected an empty target set in ' + path)
open(path, 'w').write(text.replace('targets: []', target))
PY
kill -HUP "$(cat "$SCRIPTS/smoke-alarm/instances/$MON/pid")"

echo "## phase 1 — defensive: it answers, but promises no forwarding"
cat >"$TMP/config.json" <<EOF
{"addr":"127.0.0.1:$IPORT","token":"$TOKEN","border_key":"$KEY","smoke_alarm":"http://127.0.0.1:$MPORT"}
EOF
"$TMP/wan-ingress" -config "$TMP/config.json" -heartbeat=100ms >"$TMP/ingress.log" 2>&1 &
ING=$!
disown "$ING" 2>/dev/null || true
UP=0
for _ in $(seq 1 40); do
	if curl -fsS --max-time 1 "http://127.0.0.1:$PPORT/healthz" >/dev/null 2>&1; then UP=1; break; fi
	sleep 0.25
done
[ "$UP" = 1 ] || { echo "FAILED: invite proxy could not resolve wan-ingress" >&2; exit 1; }
echo "   visitor:       healthz answered through the invite, not a URL"
sleep 3
"$ADHD_SH" up "$DASH" --smoke-alarm-instance "$MON" >/dev/null || exit 1
DPORT="$(cat "$SCRIPTS/adhd/instances/$DASH/port")"

echo "   $(grep -m1 'registered name=' "$TMP/ingress.log" | cut -c1-160)"
wait_for_text "degraded alarm target" 'wan-ingress=degraded' targets "$MPORT" >/dev/null || exit 1
LIGHTS="$(wait_for_text 'yellow light' 'wan-ingress=yellow' lights "$DPORT")" || exit 1
echo "   registry:       $(registry "$MPORT")"
echo "   alarm target:   $(targets "$MPORT")"
echo "   adhd lights:    $LIGHTS"

echo "## phase 2 — SIGHUP adds an agreed route; a real forward turns the light green"
python3 -m http.server "$BPORT" --bind 127.0.0.1 >"$TMP/border.log" 2>&1 &
BORDER=$!
sleep 1
cat >"$TMP/agreements.json" <<EOF
{"agreements":[{"id":"42","namespace":"session-42","border":"http://127.0.0.1:$BPORT","aliases":["42"],"rung":2}]}
EOF
cat >"$TMP/config.json" <<EOF
{"addr":"127.0.0.1:$IPORT","agreements":"$TMP/agreements.json","token":"$TOKEN","border_key":"$KEY","smoke_alarm":"http://127.0.0.1:$MPORT"}
EOF
kill -HUP "$ING"
sleep 1
echo "   forward:        $(curl -sS "http://127.0.0.1:$PPORT/forward?name=42" -X POST -d payload | json)  (no caller token — the invite grants it)"
wait_for_text "healthy alarm target" 'wan-ingress=healthy' targets "$MPORT" >/dev/null || exit 1
LIGHTS="$(wait_for_text 'green light' 'wan-ingress=green' lights "$DPORT")" || exit 1
echo "   alarm target:   $(targets "$MPORT")"
echo "   adhd lights:    $LIGHTS"

echo "## phase 3 — the process goes away: red"
kill "$ING"
ING=""
wait_for_text "failed alarm target" 'wan-ingress=\(regression\|unhealthy\|outage\)' targets "$MPORT" >/dev/null || exit 1
LIGHTS="$(wait_for_text 'red light' 'wan-ingress=red' lights "$DPORT")" || exit 1
echo "   alarm target:   $(targets "$MPORT")"
echo "   adhd lights:    $LIGHTS"

echo
echo "## the transition, as adhd recorded it"
TRANSITIONS="$(grep -h '"type":"light-update"' "$LOG" 2>/dev/null | python3 -c 'import sys,json
for l in sys.stdin:
 d=json.loads(l)
 if "wan-ingress" in str(d.get("target_id")):
  print((d.get("old_status") or "?")+" -> "+(d.get("new_status") or "?"))')"
printf '%s\n' "$TRANSITIONS" | sed 's/^/   /'
for hop in 'dark -> yellow' 'yellow -> green' 'green -> red'; do
	grep -qF "$hop" <<<"$TRANSITIONS" || { echo "FAILED to observe $hop" >&2; exit 1; }
done
echo
echo "PASS: defensive yellow -> forwarding green -> dead red, as judged by smoke-alarm and adhd"
