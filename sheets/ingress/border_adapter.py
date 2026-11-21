#!/usr/bin/env python3
"""Harness border for the blue-team adhd-proxy test.

wan-ingress never chooses a destination: it signs an envelope to the agreed
border. This adapter is that border. It verifies the HMAC exactly as the
forwarder builds it, then hands the opaque JSON-RPC body to a target adhd MCP
surface. It is deliberately not part of the shipped binary.
"""
from __future__ import annotations

import argparse
import hashlib
import hmac
import http.server
import json
import urllib.error
import urllib.parse
import urllib.request


def verify(key: bytes, path: str, namespace: str, body: bytes, headers) -> bool:
    signature = headers.get("X-Lab-Signature", "")
    timestamp = headers.get("X-Lab-Timestamp", "")
    if not signature or not timestamp:
        return False
    digest = hashlib.sha256(body).hexdigest()
    escaped = urllib.parse.quote(path, safe="/")
    expected = hmac.new(
        key,
        ("POST\n%s\n%s\n%s\n%s" % (escaped, namespace, digest, timestamp)).encode(),
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(signature, expected)


def make_handler(key: bytes, target: str):
    class Border(http.server.BaseHTTPRequestHandler):
        def do_POST(self):  # noqa: N802
            length = int(self.headers.get("Content-Length", "0") or 0)
            body = self.rfile.read(length)
            namespace = self.headers.get("X-Lab-Namespace", "")
            if not verify(key, self.path, namespace, body, self.headers):
                self.send_response(403)
                self.end_headers()
                self.wfile.write(b"bad border signature")
                return

            req = urllib.request.Request(
                target,
                data=body,
                headers={"Content-Type": "application/json", "Accept": "application/json, text/event-stream"},
            )
            try:
                resp = urllib.request.urlopen(req, timeout=8)
                status, payload = resp.status, resp.read()
            except urllib.error.HTTPError as e:
                status, payload = e.code, e.read()
            except Exception as e:  # pragma: no cover - harness diagnosis only
                status, payload = 502, json.dumps({"error": str(e)}).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, fmt, *args):
            pass

    return Border


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:0")
    ap.add_argument("--key", required=True)
    ap.add_argument("--target", required=True, help="target adhd /mcp endpoint")
    args = ap.parse_args()

    host, port = args.addr.rsplit(":", 1)
    server = http.server.ThreadingHTTPServer((host, int(port)), make_handler(args.key.encode(), args.target))
    print("border listening %s -> %s" % (server.server_address, args.target), flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
