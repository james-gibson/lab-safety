#!/bin/bash
#
# tripwire.sh — the honeypot tripwire, observed by a *second* adhd.
#
# Model: the honeypotted node must not be the detector (it can't be trusted to
# report on itself), so detection takes a separate adhd that watches the alarm.
# The honeypot host fires the alert; the alarm surfaces it; the observer turns a
# light red. See ./TRIPWIRE.md.
#
# The decoy's **sensitivity** is a 42i distance, not a switch. Every contact is
# graded by how it was made — a tools/call at the decoy costs 16 (the advertised
# route with no stamp for it), a direct method call at its name costs 32 (a name
# invoked as a method the surface never offered) — and the cost accumulates per
# caller at the host. The alert fires when the caller's distance reaches the
# threshold, so one caller is tolerated at 32 and alerted at 48.
#
#   TRIPWIRE_SENSITIVITY=48   both contacts (the default): tolerate, then fire
#   TRIPWIRE_SENSITIVITY=32   the direct method call alone fires it
#   TRIPWIRE_SENSITIVITY=16   the first tools/call fires it
#   TRIPWIRE_SENSITIVITY=0    a silent honeypot: any contact fires (the old behaviour)
#
# Every contact is logged whether or not it fires, so "no alert" here means "seen,
# and below the threshold" — not "blind".
#
# Builds the local (fixed) binaries so the exercise reflects the working tree,
# not the release binaries. Touches only ephemeral spin instances.
#
set -euo pipefail

SCRIPTS="${KB_SCRIPTS:-$HOME/projects/kb/scripts}"
SMOKE_REPO="${SMOKE_REPO:-$HOME/projects/personal/smoke-alarm}"
ADHD_REPO="${ADHD_REPO:-$HOME/projects/personal/adhd}"
BIN="${BIN:-/tmp/kb-lab-tripwire}"
SENSITIVITY="${TRIPWIRE_SENSITIVITY:-48}"

ALARM="$SCRIPTS/smoke-alarm/spin.sh"
ADHD="$SCRIPTS/adhd/spin.sh"
[ -x "$ALARM" ] || { echo "smoke-alarm spin.sh not found at $ALARM (set KB_SCRIPTS)" >&2; exit 1; }
[ -x "$ADHD" ] || { echo "adhd spin.sh not found at $ADHD (set KB_SCRIPTS)" >&2; exit 1; }
case "$SENSITIVITY" in
	''|*[!0-9]*) echo "TRIPWIRE_SENSITIVITY must be a non-negative 42i distance (e.g. 0, 16, 32, 48)" >&2; exit 1 ;;
esac

HPLOG="$HOME/Library/Logs/kb-adhd-hp.err"

# Only the instances this run started. The vault's instances/ tree is shared with
# whatever else is spun up, so nothing here removes a directory.
cleanup() {
	"$ADHD"  down obs      >/dev/null 2>&1 || true
	"$ADHD"  down hp       >/dev/null 2>&1 || true
	"$ALARM" down sentinel >/dev/null 2>&1 || true
}
trap cleanup EXIT
"$ADHD"  down obs      >/dev/null 2>&1 || true
"$ADHD"  down hp       >/dev/null 2>&1 || true
"$ALARM" down sentinel >/dev/null 2>&1 || true

security_of() { curl -s --max-time 3 "http://127.0.0.1:$1/status" | python3 -c 'import sys,json
d=json.load(sys.stdin)
c=[x for x in d.get("components",[]) if "security" in x["name"]]
print("; ".join("%s healthy=%s — %s" % (x["name"], x["healthy"], x.get("detail","")) for x in c) or "(none)")'; }
contact_cost() { grep -h "honeypot tool triggered" "$HPLOG" 2>/dev/null | tail -1 \
	| grep -o 'via=[^ ]* .*fired=[a-z]*' || echo "(no contact seen)"; }

echo "## build fixed binaries from the working tree"
mkdir -p "$BIN"
(cd "$SMOKE_REPO" && go build -o "$BIN/ocd-smoke-alarm" ./cmd/ocd-smoke-alarm)
(cd "$ADHD_REPO"  && go build -o "$BIN/adhd"           ./cmd/adhd)
export ALARM_BIN="$BIN/ocd-smoke-alarm" ADHD_BIN="$BIN/adhd"

echo
echo "## sentinel — the trust authority / alert sink"
"$ALARM" up sentinel --empty
SH="$(cat "$SCRIPTS/smoke-alarm/instances/sentinel/port")"

echo
echo "## hp — the honeypotted node (reports, but does not observe)"
: >"$HPLOG" 2>/dev/null || true
"$ADHD" up hp --smoke-alarm "http://127.0.0.1:$SH"
HP="$(cat "$SCRIPTS/adhd/instances/hp/port")"

echo
echo "## obs — a SECOND adhd, the observer (this is what turns red)"
"$ADHD" up obs --smoke-alarm "http://127.0.0.1:$SH"
OP="$(cat "$SCRIPTS/adhd/instances/obs/port")"
sleep 2

echo
echo "## the honeypot (bait) on hp, registered via cluster.join"
echo "   sensitivity $SENSITIVITY — the 42i distance at which a caller is an alert"
JOIN="$(curl -s --max-time 4 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' -d @- <<EOF
{"jsonrpc":"2.0","id":1,"method":"adhd.cluster.join","params":{
  "name":"probe-cluster",
  "honeypot-tools":[{"name":"adhd.project.canary.call",
                     "description":"internal project proxy (do not call)",
                     "sensitivity":$SENSITIVITY}]}}
EOF
)"
printf '%s' "$JOIN" | python3 -m json.tool

# The host says back what it armed. If it does not, it predates the threshold and
# would fire on the first contact — say so here rather than let the run appear to
# demonstrate tolerance that is not there.
ARMED="$(printf '%s' "$JOIN" | python3 -c 'import sys,json
h=(json.load(sys.stdin).get("result") or {}).get("honeypots") or []
print(h[0].get("sensitivity") if h else "")' 2>/dev/null || true)"
if [ "$ARMED" != "$SENSITIVITY" ]; then
	echo "   NOTE: the host armed ${ARMED:-no threshold}; this adhd predates a configurable" >&2
	echo "         sensitivity (james-gibson/adhd#61) and will fire on the first contact." >&2
fi

echo
echo "## TRUSTED caller — uses only the pre-supplied tools (clean, no distance)"
for m in adhd.status adhd.lights.list; do
	printf '  obs %s -> ' "$m"
	curl -s --max-time 3 -XPOST "http://127.0.0.1:$OP/mcp" -H 'Content-Type: application/json' \
		-d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$m\"}" | head -c 80; echo
done
echo "   hp: $(contact_cost)   (nothing: the trusted caller never touches bait)"

echo
echo "## PROBING caller — two contacts, graded"
printf '  direct method (+32)  -> '
curl -s --max-time 3 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.project.canary.call"}'; echo
COST="$(contact_cost)"; echo "     hp:    $COST"
case "$COST" in
	*fired=true*) echo "     alarm: $(security_of "$SH")" ;;
	*)            echo "     alarm: $(security_of "$SH")   (seen by the host, below the threshold)" ;;
esac

printf '  tools/call (+16)     -> '
curl -s --max-time 3 -XPOST "http://127.0.0.1:$HP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"adhd.project.canary.call"}}'; echo
echo "     hp:    $(contact_cost)"
sleep 1

echo
echo "## sentinel: the alert surfaces (smoke-alarm#23 fixed)"
echo "   $(security_of "$SH")"

echo
echo "## obs: the observer goes red (waiting for its next poll)"
sleep 13
curl -s --max-time 3 -XPOST "http://127.0.0.1:$OP/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.lights.list"}' \
	| python3 -c 'import sys,json;ls=[l for l in json.load(sys.stdin)["result"]["lights"] if "security" in l["name"]];print("\n".join("   %s %s — %s" % (l["status"].upper(), l["name"], l.get("details")) for l in ls) or "   (no security light)")'

echo
echo "## teardown"
