#!/usr/bin/env sh
# One static file per OS/arch and binary. No CGO, no runtime — each install is a copy.
set -eu
cd "$(dirname "$0")/.."

out=dist
mkdir -p "$out"
rm -f "$out"/wan-ingress-* "$out"/seed-invite-* "$out"/seed-announce-* "$out"/SHA-256SUMS

for os in darwin linux; do
  for arch in amd64 arm64; do
    for cmd in wan-ingress seed-invite seed-announce; do
      bin="$out/$cmd-$os-$arch"
      CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
        go build -trimpath -ldflags="-s -w" -o "$bin" "./cmd/$cmd"
      echo "built $bin"
    done
  done
done

if command -v shasum >/dev/null 2>&1; then
  (cd "$out" && shasum -a 256 wan-ingress-* seed-invite-* seed-announce-* > SHA-256SUMS)
else
  (cd "$out" && sha256sum wan-ingress-* seed-invite-* seed-announce-* > SHA-256SUMS)
fi
cat "$out/SHA-256SUMS"
