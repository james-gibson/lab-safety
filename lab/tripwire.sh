#!/bin/bash
#
# tripwire.sh — the honeypot tripwire, observed by a *second* adhd.
#
# Model: the honeypotted node must not be the detector (it can't be trusted to
# report on itself), so detection takes a separate adhd that watches the alarm.
# The honeypot host fires the alert; the alarm surfaces it; the observer turns a
# light red. See ./TRIPWIRE.md.
#
# Builds the local (fixed) binaries so the exercise reflects the working tree,
# not the release binaries. Touches only ephemeral spin instances.
#
set -euo pipefail

SCRIPTS="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SMOKE_REPO="${SMOKE_REPO:-$HOME/projects/personal/smoke-alarm}"
ADHD_REPO="${ADHD_REPO:-$HOME/projects/personal/adhd}"
BIN="${BIN:-/tmp/kb-lab-tripwire}"

echo "## build fixed binaries from the working tree"
mkdir -p "$BIN"
(cd "$SMOKE_REPO" && go build -o "$BIN/ocd-smoke-alarm" ./cmd/ocd-smoke-alarm)
(cd "$ADHD_REPO"  && go build -o "$BIN/adhd"           ./cmd/adhd)
export ALARM_BIN="$BIN/ocd-smoke-alarm" ADHD_BIN="$BIN/adhd"

ALARM="$SCRIPTS/smoke-alarm/spin.sh"
ADHD="$SCRIPTS/adhd/spin.sh"
cleanup() {
	"$ADHD"  down obs      >/dev/null 2>&1 || true
	"$ADHD"  down hp       >/dev/null 2>&1 || true
	"$ALARM" down sentinel >/dev/null 2>&1 || true
	rm -rf "$SCRIPTS/adhd/instances" "$SCRIPTS/smoke-alarm/instances"
}
trap cleanup EXIT

echo
echo "## sentinel — the trust authority / alert sink"
"$ALARM" up sentinel --empty
SH="$(cat "$SCRIPTS/smoke-alarm/instances/sentinel/port")"

echo
echo "## hp — the honeypotted node (reports, but does not observe)"
"$ADHD" up hp --smoke-alarm "http://127.0.0.1:$SH"
HP="$(cat "$SCRIPTS/adhd/instances/hp/port")"

echo
echo "## obs — a SECOND adhd, the observer (this is what turns red)"
"$ADHD" up obs --smoke-alarm "http://127.0.0.1:$SH"
OP="$(cat "$SCRIPTS/adhd/instances/obs/port")"
sleep 2

echo
echo "## the honeypot (bait) on hp, registered via cluster.join"
curl -s --max-time 4 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.cluster.join","params":{"name":"probe-cluster","honeypot-tools":[{"name":"adhd.project.canary.call","description":"internal project proxy (do not call)"}]}}'; echo

echo
echo "## TRUSTED caller — uses only the pre-supplied tools (clean)"
for m in adhd.status adhd.lights.list; do
	printf '  obs %s -> ' "$m"
	curl -s --max-time 3 -XPOST "http://127.0.0.1:$OP/mcp" -H 'Content-Type: application/json' \
		-d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$m\"}" | head -c 80; echo
done

echo
echo "## PROBING caller — explores, touches the honeypot on hp"
printf '  direct method -> '
curl -s --max-time 3 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.project.canary.call"}'; echo
printf '  tools/call    -> '
curl -s --max-time 3 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"adhd.project.canary.call"}}'; echo
sleep 1

echo
echo "## hp: the tripwire fires (hp logs, but does not observe)"
grep -i "honeypot" "$HOME/Library/Logs/kb-adhd-hp.err" | tail -2

echo
echo "## sentinel: the alert surfaces (smoke-alarm#23 fixed)"
curl -s --max-time 3 "http://127.0.0.1:$SH/status" \
	| python3 -c 'import sys,json;d=json.load(sys.stdin);[print("  ",c["name"],"healthy="+str(c["healthy"]),"-",c.get("detail","")) for c in d.get("components",[]) if "security" in c["name"] or c["name"]=="federation"]'

echo
echo "## obs: the observer goes red (waiting for its next poll)"
sleep 13
curl -s --max-time 3 -XPOST "http://127.0.0.1:$OP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' \
	| python3 -c 'import sys,json;[print("  ",l["status"].upper(),l["name"],"-",l.get("details")) for l in json.load(sys.stdin)["result"]["lights"] if "security" in l["name"]]'

echo
echo "## teardown"
