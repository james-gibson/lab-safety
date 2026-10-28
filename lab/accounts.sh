#!/bin/bash
#
# accounts.sh — the verification chain: ingress → verifier → monitor → …
#
# Alarm A is ingress and sheds load to B on its own choice; B is idle but is
# allowed to check A's work (traffic mirroring); B is monitored by C; and so on.
# Each level verifies the one below, so a busy node cannot self-report — an idle
# sibling checks it, and that sibling is checked in turn. The honeypot probe is
# where the isotopes settle the accounts. See ./RECONCILE.md.
#
# Builds the working-tree binaries. Touches only /tmp state.
#
set -euo pipefail

SMOKE_REPO="${SMOKE_REPO:-$HOME/projects/personal/smoke-alarm}"
ADHD_REPO="${ADHD_REPO:-$HOME/projects/personal/adhd}"
BIN="${BIN:-/tmp/kb-lab-tripwire}"
STATE="${STATE:-/tmp/kb-lab-accounts}"

free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }
port_of() { case "$1" in a) echo "$PA" ;; b) echo "$PB" ;; c) echo "$PC" ;; esac; }

echo "## build working-tree binaries"
mkdir -p "$BIN" "$STATE"
(cd "$SMOKE_REPO" && go build -o "$BIN/ocd-smoke-alarm" ./cmd/ocd-smoke-alarm)
(cd "$ADHD_REPO"  && go build -o "$BIN/adhd"           ./cmd/adhd)

PA="$(free_port)"; PB="$(free_port)"; PC="$(free_port)"; PH="$(free_port)"

# alarm_cfg <file> <name> <port> <verify-port-or-empty>
# Empty verify-port = ingress (nothing above it); otherwise a `verify` target
# that checks the work of the level below.
alarm_cfg() {
	local file="$1" name="$2" port="$3" below="$4"
	{
		cat <<EOF
version: "1"
service: { name: "$name", environment: "accounts", mode: "foreground", log_level: "info", poll_interval: "5s", timeout: "3s" }
health: { enabled: true, listen_addr: "127.0.0.1:$port", endpoints: { healthz: "/healthz", readyz: "/readyz", status: "/status" } }
runtime: { lock_file: "$STATE/$name.lock", state_dir: "$STATE/$name", baseline_file: "$STATE/$name/known-good.json", graceful_shutdown_timeout: "3s" }
discovery: { enabled: false }
alerts: { aggressive: false, sinks: { log: { enabled: true } } }
auth: { keystore: { enabled: false }, redaction: { enabled: true, mask: "****" }, oauth: { enabled: false } }
targets:
EOF
		if [ -n "$below" ]; then
			cat <<EOF
  - id: "verify"
    enabled: true
    protocol: "http"
    name: "verify $below"
    endpoint: "http://127.0.0.1:$below/status"
    transport: "http"
    expected: { healthy_status_codes: [200] }
    check: { interval: "5s", timeout: "3s", retries: 0, handshake_profile: "none" }
EOF
		else
			printf '  []\n'
		fi
		cat <<EOF
known_state: { enabled: true, persist: false }
hosted: { enabled: false }
federation: { enabled: true, base_port: 6400, max_port: 6410 }
EOF
	} > "$file"
}

alarm_cfg "$STATE/a.yaml" "alarm-a" "$PA" ""      # ingress: nothing below it
alarm_cfg "$STATE/b.yaml" "alarm-b" "$PB" "$PA"   # verifier: checks A
alarm_cfg "$STATE/c.yaml" "alarm-c" "$PC" "$PB"   # monitor: checks B

echo "## chain: A (ingress) <- B (verifier) <- C (monitor)"
for n in a b c; do
	nohup "$BIN/ocd-smoke-alarm" serve --config="$STATE/$n.yaml" --mode=foreground -tui=false >"$STATE/$n.log" 2>&1 &
	echo $! > "$STATE/$n.pid"
done

# The honeypot host reports to A alone — the ingress. In the design, B learns by
# checking A's work, and C by checking B's; that is the upstream push (see below).
cat > "$STATE/host.yaml" <<EOF
mcp_server: { enabled: true, addr: "127.0.0.1:$PH" }
smoke_alarm:
  - { name: "alarm-a", endpoint: "http://127.0.0.1:$PA", interval: 10s, use_sse: false }
EOF
echo "## honeypot host reports to A (the ingress) only"
nohup "$BIN/adhd" --headless --config "$STATE/host.yaml" >"$STATE/host.log" 2>&1 & echo $! > "$STATE/host.pid"
sleep 3

cleanup() {
	for p in host a b c; do [ -f "$STATE/$p.pid" ] && kill "$(cat "$STATE/$p.pid")" 2>/dev/null || true; done
	rm -f "$STATE"/*.pid
}
trap cleanup EXIT

echo
echo "## the chain checks each other's work"
printf '  B verifies A -> '; curl -s --max-time 3 "http://127.0.0.1:$PB/status" | python3 -c 'import sys,json;d=json.load(sys.stdin);print([(t["id"],t["state"]) for t in d.get("targets",[])])'
printf '  C verifies B -> '; curl -s --max-time 3 "http://127.0.0.1:$PC/status" | python3 -c 'import sys,json;d=json.load(sys.stdin);print([(t["id"],t["state"]) for t in d.get("targets",[])])'

echo
echo "## prober touches the bait at A's ingress"
curl -s --max-time 4 -XPOST "http://127.0.0.1:$PH/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.cluster.join","params":{"name":"probe-cluster","honeypot-tools":[{"name":"adhd.project.canary.call","description":"internal proxy (do not call)"}]}}' >/dev/null
curl -s --max-time 3 -XPOST "http://127.0.0.1:$PH/mcp" -H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","id":1,"method":"adhd.project.canary.call"}' >/dev/null
sleep 2

echo
echo "## the accounts"
for x in a b c; do
	port="$(port_of "$x")"
	printf '  %s: ' "$x"
	curl -s --max-time 3 "http://127.0.0.1:$port/status" \
		| python3 -c 'import sys,json;d=json.load(sys.stdin);print([(c["name"],"healthy="+str(c["healthy"])) for c in d.get("components",[]) if "security" in c["name"]] or "no security component")'
done

echo
echo "## teardown"
