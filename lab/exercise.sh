#!/bin/bash
#
# exercise.sh — drive the tuner + smoke-alarm + adhd flow and print the event
# trail. Annotated result: ./FLOW.md. Touches only ephemeral spin instances; the
# idle launchd adhd on :9090 is the endpoint under test.
#
# Usage: ./exercise.sh
#
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TUNER="$SCRIPTS/tuner/spin.sh"
ALARM="$SCRIPTS/smoke-alarm/spin.sh"
ADHD="$SCRIPTS/adhd/spin.sh"
ADHD_MCP="${ADHD_MCP:-http://127.0.0.1:9090/mcp}"

cleanup() {
	"$ADHD"  down dash >/dev/null 2>&1 || true
	"$ALARM" down flow >/dev/null 2>&1 || true
	"$TUNER" down t0   >/dev/null 2>&1 || true
	rm -rf "$SCRIPTS/tuner/instances" "$SCRIPTS/smoke-alarm/instances" "$SCRIPTS/adhd/instances"
}
trap cleanup EXIT

echo "## 1. tuner — broadcaster"
"$TUNER" up t0
TB="$(cat "$SCRIPTS/tuner/instances/t0/port")"

echo
echo "## 2. smoke-alarm — monitor, tuner-integrated, watching the idle adhd"
"$ALARM" up flow --tuner --target "$ADHD_MCP"
AH="$(cat "$SCRIPTS/smoke-alarm/instances/flow/port")"
AHO="$(cat "$SCRIPTS/smoke-alarm/instances/flow/hosted-port")"

echo
echo "## 3. adhd — dashboard, watching the alarm"
"$ADHD" up dash --smoke-alarm "http://127.0.0.1:$AH"
AD="$(cat "$SCRIPTS/adhd/instances/dash/port")"

sleep 3
echo
echo "## tuner channels"
curl -s --max-time 3 "http://127.0.0.1:$TB/channels"; echo
echo "## alarm status (targets)"
curl -s --max-time 3 "http://127.0.0.1:$AH/status" \
	| python3 -c 'import sys,json;d=json.load(sys.stdin);print("ready",d["ready"]);[print("  ",t["id"],t["state"],"-",t.get("message")) for t in d.get("targets",[])]'

echo
echo "## drive: tuner -> alarm  (audience)"
curl -s --max-time 3 -XPOST "http://127.0.0.1:$AHO/tuner/audience" \
	-H 'Content-Type: application/json' -d '{"channel":"ntp","count":3,"signal":0.81}'; echo
echo "## drive: tuner -> alarm  (caller)"
curl -s --max-time 3 -XPOST "http://127.0.0.1:$AHO/tuner/caller/ntp" \
	-H 'Content-Type: application/json' -d '{"message":"clock drift rising","from":"tuner","priority":"warn"}'; echo

echo
echo "## alarm audience state"
curl -s --max-time 3 "http://127.0.0.1:$AHO/tuner/audience" | python3 -m json.tool
echo "## alarm hosted events (protocol=tuner)"
curl -sN --max-time 2 "http://127.0.0.1:$AHO/hosted/events?transport=sse" | grep '^data:' | head -4 || true

echo
echo "## adhd lights (after a poll cycle)"
sleep 4
curl -s --max-time 3 -XPOST "http://127.0.0.1:$AD/mcp" \
	-H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"adhd.status"}'; echo
echo "## adhd traffic log"
tail -n 8 "$HOME/Library/Logs/kb-adhd-dash.jsonl"

echo
echo "## teardown"
