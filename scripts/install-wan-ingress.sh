#!/bin/bash
#
# Install wan-ingress as a supervised service. The binary stays a single static
# file; restart policy belongs to the host, where it survives a crash of the
# process itself. SIGHUP reloads policy without a restart, so adding traffic is
# not a reason to stop the service.
#
#   BINARY=./dist/wan-ingress \
#   CONFIG=./configs/wan-ingress.json \
#   ./scripts/install-wan-ingress.sh
#
# CONFIG is deliberately required. There is no ready-made policy: copy
# configs/wan-ingress.example.json, fill in the operator-owned token, border key,
# agreement path and registry, then install that file.
#
# Override HOME/LOGDIR to install somewhere other than the user's agents.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${BINARY:-$HERE/../dist/wan-ingress}"
LOGDIR="${LOGDIR:-$HOME/Library/Logs}"

[ -n "${CONFIG:-}" ] || { echo "CONFIG is required; copy configs/wan-ingress.example.json and fill it in" >&2; exit 1; }
[ -f "$CONFIG" ] || { echo "config not found: $CONFIG" >&2; exit 1; }
[ -x "$BINARY" ] || { echo "binary not found or not executable: $BINARY" >&2; exit 1; }
BINARY="$(cd "$(dirname "$BINARY")" && pwd)/$(basename "$BINARY")"
CONFIG="$(cd "$(dirname "$CONFIG")" && pwd)/$(basename "$CONFIG")"
mkdir -p "$LOGDIR"

case "$(uname -s)" in
  Darwin)
    DEST="${HOME}/Library/LaunchAgents/co.james-gibson.lab.wan-ingress.plist"
    sed -e "s|__BINARY__|$BINARY|g" -e "s|__CONFIG__|$CONFIG|g" \
        -e "s|__LOGDIR__|$LOGDIR|g" "$HERE/wan-ingress.plist" > "$DEST"
    launchctl unload "$DEST" 2>/dev/null || true
    launchctl load "$DEST"
    echo "installed $DEST (KeepAlive: crash restarts the ingress)"
    ;;
  Linux)
    DEST="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/wan-ingress.service"
    mkdir -p "$(dirname "$DEST")"
    sed -e "s|__BINARY__|$BINARY|g" -e "s|__CONFIG__|$CONFIG|g" \
        -e "s|__LOGDIR__|$LOGDIR|g" "$HERE/wan-ingress.service" > "$DEST"
    systemctl --user daemon-reload
    systemctl --user enable --now wan-ingress.service
    echo "installed $DEST (Restart=always: crash restarts the ingress)"
    ;;
  *)
    echo "unsupported host: $(uname -s); install the unit manually from $HERE" >&2
    exit 1
    ;;
esac
