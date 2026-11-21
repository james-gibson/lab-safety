#!/usr/bin/env python3
"""Harness border for the blue-team adhd-proxy tests.

wan-ingress never chooses a destination: it signs an envelope to the agreed
border. This adapter is that border. It verifies the HMAC exactly as the
forwarder builds it, then hands the opaque JSON-RPC body to a target adhd MCP
surface. It is deliberately not part of the shipped binary.

`--connection-mode hold` keeps both the inbound (ingress → border) and outbound
(border → target) HTTP connections open across calls. `--connection-mode new`
answers with `Connection: close` and opens a fresh outbound connection per call,
so the same test can measure what connection churn costs.
"""
from __future__ import annotations

import argparse
import hashlib
import hmac
import http.client
import http.server
import json
import threading
import urllib.parse


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


class BorderState:
    def __init__(self, mode: str, target: str):
        self.mode = mode
        parts = urllib.parse.urlsplit(target)
        if parts.scheme != "http" or not parts.hostname:
            raise ValueError("border target must be an http URL")
        self.host = parts.hostname
        self.port = parts.port or 80
        self.path = parts.path or "/"
        self.lock = threading.Lock()
        self.requests = 0
        self.inbound: set[tuple] = set()
        self.outbound = 0
        self._conn: http.client.HTTPConnection | None = None

    def _connect(self) -> http.client.HTTPConnection:
        conn = http.client.HTTPConnection(self.host, self.port, timeout=8)
        conn.connect()
        return conn

    def forward(self, body: bytes) -> tuple[int, bytes]:
        with self.lock:
            self.requests += 1
            fresh = self.mode == "new" or self._conn is None
            if fresh:
                conn = self._connect()
                self.outbound += 1
                if self.mode == "hold":
                    self._conn = conn
            else:
                conn = self._conn

        headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
        for attempt in range(2):
            try:
                conn.request("POST", self.path, body=body, headers=headers)
                resp = conn.getresponse()
                status, payload = resp.status, resp.read()
                if self.mode == "new":
                    conn.close()
                return status, payload
            except Exception as exc:
                conn.close()
                with self.lock:
                    self._conn = None
                    if attempt == 0:
                        conn = self._connect()
                        self.outbound += 1
                        if self.mode == "hold":
                            self._conn = conn
                        continue
                return 502, json.dumps({"error": str(exc)}).encode()
        return 502, b"border forward failed"

    def stats(self) -> dict:
        with self.lock:
            return {
                "mode": self.mode,
                "requests": self.requests,
                "inbound_connections": len(self.inbound),
                "outbound_connections": self.outbound,
            }

    def reset(self):
        with self.lock:
            self.requests = 0
            self.inbound = set()
            self.outbound = 0
            if self._conn is not None:
                self._conn.close()
                self._conn = None


def make_handler(key: bytes, state: BorderState):
    class Border(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def _json(self, status: int, payload: dict):
            body = json.dumps(payload).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            if state.mode == "new":
                self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):  # noqa: N802
            if self.path == "/stats":
                self._json(200, state.stats())
            elif self.path == "/stats/reset":
                state.reset()
                self._json(200, state.stats())
            else:
                self._json(404, {"error": "not found"})

        def do_POST(self):  # noqa: N802
            with state.lock:
                state.inbound.add(self.client_address)
            length = int(self.headers.get("Content-Length", "0") or 0)
            body = self.rfile.read(length)
            namespace = self.headers.get("X-Lab-Namespace", "")
            if not verify(key, self.path, namespace, body, self.headers):
                self._json(403, {"error": "bad border signature"})
                return
            status, payload = state.forward(body)
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            if state.mode == "new":
                self.send_header("Connection", "close")
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
    ap.add_argument("--connection-mode", choices=("hold", "new"), default="hold")
    args = ap.parse_args()

    host, port = args.addr.rsplit(":", 1)
    state = BorderState(args.connection_mode, args.target)
    server = http.server.ThreadingHTTPServer((host, int(port)), make_handler(args.key.encode(), state))
    print("border %s listening %s -> %s" % (args.connection_mode, server.server_address, args.target), flush=True)
    server.serve_forever()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
