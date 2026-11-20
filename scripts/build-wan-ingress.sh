#!/usr/bin/env sh
# One static file per OS/arch. No CGO, no runtime, no sidecar — the install is a copy.
set -eu
cd "$(dirname "$0")/.."

out=dist
mkdir -p "$out"
rm -f "$out"/wan-ingress-* "$out"/SHA-256SUMS

for os in darwin linux; do
  for arch in amd64 arm64; do
    bin="$out/wan-ingress-$os-$arch"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags="-s -w" -o "$bin" ./cmd/wan-ingress
    echo "built $bin"
  done
done

if command -v shasum >/dev/null 2>&1; then
  (cd "$out" && shasum -a 256 wan-ingress-* > SHA-256SUMS)
else
  (cd "$out" && sha256sum wan-ingress-* > SHA-256SUMS)
fi
cat "$out/SHA-256SUMS"
