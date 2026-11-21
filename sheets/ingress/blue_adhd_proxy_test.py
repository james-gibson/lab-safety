#!/usr/bin/env python3
"""Blue-team test: two adhd proxies, one ingress, one measured baseline.

Prediction-first claim: the boundary is a trust boundary, not a speed trick.
Both caller surfaces are the same adhd binary and both call the same target
through `adhd.proxy`; only one path crosses wan-ingress. The signed border must
accept exactly that path and reject an unsigned copy. Latency is reported as the
cost blue pays, never as the finding.

Harness-only. Nothing here ships in the ingress binary and the Gherkin origin is
unchanged. Run from the repo root:

    python3 sheets/ingress/blue_adhd_proxy_test.py --calls 60
"""
from __future__ import annotations

import argparse
import importlib.util
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
SCRIPTS = os.environ.get("KB_SCRIPTS", "/Users/dev/projects/kb/scripts")
ALARM_SH = os.path.join(SCRIPTS, "smoke-alarm", "spin.sh")
ADHD_SH = os.path.join(SCRIPTS, "adhd", "spin.sh")
BLUE_LATENCY = os.path.join(SCRIPTS, "lab", "redteam", "blue-latency.py")
ACCEPT = "application/json, text/event-stream"

TOKEN = "blue-invite-token"
BORDER_KEY = "blue-border-key"
AGREEMENT_ALIAS = "42"
NAMESPACE = "session-42"


def free_port() -> int:
    s = socket.socket()
    s.bind(("127.0.0.1", 0))
    port = s.getsockname()[1]
    s.close()
    return port


def post(url: str, payload: dict, extra_headers: dict | None = None, timeout: float = 12.0):
    headers = {"Content-Type": "application/json", "Accept": ACCEPT}
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers=headers)
    t0 = time.perf_counter()
    try:
        resp = urllib.request.urlopen(req, timeout=timeout)
        status, body = resp.status, resp.read().decode()
    except urllib.error.HTTPError as e:
        status, body = e.code, e.read().decode()
    return (time.perf_counter() - t0) * 1000.0, status, body


def get(url: str, timeout: float = 3.0):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            return resp.status, resp.read().decode()
    except Exception:
        return 0, ""


def wait_for(label: str, check, timeout: float = 25.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if check():
            return
        time.sleep(0.25)
    raise RuntimeError("timed out waiting for " + label)


def tcp_open(port: int) -> bool:
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.3):
            return True
    except OSError:
        return False


def rpc(url: str, method: str, rid: int = 1, params: dict | None = None):
    payload = {"jsonrpc": "2.0", "id": rid, "method": method}
    if params is not None:
        payload["params"] = params
    return post(url, payload)


def adhd_proxy_call(caller_url: str, target_endpoint: str, method: str, rid: int):
    """Both callers use the same tool; only target_endpoint differs."""
    return post(
        caller_url,
        {
            "jsonrpc": "2.0",
            "id": rid,
            "method": "tools/call",
            "params": {
                "name": "adhd.proxy",
                "arguments": {
                    "target_endpoint": target_endpoint,
                    "call": {"method": method},
                },
            },
        },
    )


def proxy_result(body: str):
    """Unwrap adhd's tools/call content[0].text into the proxy response."""
    try:
        outer = json.loads(body)
        text = outer["result"]["content"][0]["text"]
        return json.loads(text)
    except Exception:
        return {}


def load_blue_latency():
    spec = importlib.util.spec_from_file_location("blue_latency", BLUE_LATENCY)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load " + BLUE_LATENCY)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def log_latencies(path: str, method: str) -> list[float]:
    """Read the adhd JSONL log with blue-latency's own baseline contract."""
    if not os.path.exists(path):
        return []
    mod = load_blue_latency()
    return mod.load_baseline(path, method)


def verdict(mod, baseline: list[float], live: list[float]) -> dict:
    pct = mod.pct
    _, _, p = mod.mannwhitney(baseline, live)
    base_p50, base_p99 = pct(baseline, 0.5), pct(baseline, 0.99)
    live_p50 = pct(live, 0.5)
    if live_p50 > base_p99:
        code, label = 2, "unmistakable (live median past baseline p99)"
    elif p < 0.05:
        code, label = 1, "detectable at this call count"
    else:
        code, label = 0, "indistinguishable at this call count"
    return {
        "baseline": {"n": len(baseline), "p50_ms": round(base_p50, 3),
                     "p95_ms": round(pct(baseline, 0.95), 3), "p99_ms": round(base_p99, 3)},
        "live": {"n": len(live), "p50_ms": round(live_p50, 3),
                 "p95_ms": round(pct(live, 0.95), 3), "p99_ms": round(pct(live, 0.99), 3)},
        "median_shift_ms": round(live_p50 - base_p50, 3),
        "p_value": round(p, 6),
        "code": code,
        "label": label,
    }


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--calls", type=int, default=60, help="calls per path (>=50 for the rank test)")
    ap.add_argument("--connection-mode", choices=("hold", "new"), default="hold",
                    help="hold: reuse ingress↔border and border↔target connections; new: close each")
    ap.add_argument("--out", default=os.path.join(tempfile.gettempdir(), "blue-adhd-proxy-baseline.json"))
    args = ap.parse_args()
    if args.calls < 50:
        ap.error("--calls must be >= 50 for the blue-latency baseline contract")

    for path in (ALARM_SH, ADHD_SH, BLUE_LATENCY):
        if not os.path.exists(path):
            ap.error("missing harness tool: " + path)

    stamp = "%d" % os.getpid()
    reg_name, target_name = "blue-reg-" + stamp, "blue-target-" + stamp
    direct_name, ingress_name = "blue-direct-" + stamp, "blue-ingress-" + stamp
    tmp = tempfile.mkdtemp(prefix="blue-adhd-")
    procs: list[subprocess.Popen] = []
    logs: dict[str, str] = {}
    result: dict = {}

    def spawn(argv: list[str], name: str, cwd: str = REPO) -> subprocess.Popen:
        log = os.path.join(tmp, name + ".log")
        handle = open(log, "wb")
        logs[name] = log
        proc = subprocess.Popen(argv, stdout=handle, stderr=subprocess.STDOUT, cwd=cwd)
        procs.append(proc)
        return proc

    def down(spin: str, name: str):
        try:
            subprocess.run([spin, "down", name], check=False, capture_output=True, timeout=15)
        except Exception:
            pass

    try:
        # Build the two harness binaries from this checkout.
        spawn(["go", "build", "-o", os.path.join(tmp, "wan-ingress"), "./cmd/wan-ingress"], "build-ingress").wait()
        spawn(["go", "build", "-o", os.path.join(tmp, "invite-proxy"), "./sheets/ingress/proxy"], "build-proxy").wait()
        if any(p.returncode for p in procs):
            raise RuntimeError("build failed: " + ", ".join(
                n for n, p in zip(("build-ingress", "build-proxy"), procs) if p.returncode))

        print("## bring up the registry, target, and two caller adhd proxies")
        subprocess.run([ALARM_SH, "up", reg_name, "--empty"], check=True, capture_output=True, timeout=60)
        subprocess.run([ADHD_SH, "up", target_name], check=True, capture_output=True, timeout=60)
        subprocess.run([ADHD_SH, "up", direct_name], check=True, capture_output=True, timeout=60)
        subprocess.run([ADHD_SH, "up", ingress_name], check=True, capture_output=True, timeout=60)

        def instance_port(script_dir: str, name: str) -> int:
            with open(os.path.join(SCRIPTS, script_dir, "instances", name, "port")) as f:
                return int(f.read().strip())

        reg_port = instance_port("smoke-alarm", reg_name)
        target_port = instance_port("adhd", target_name)
        direct_port = instance_port("adhd", direct_name)
        ingress_port = instance_port("adhd", ingress_name)
        registry_url = "http://127.0.0.1:%d" % reg_port
        target_mcp = "http://127.0.0.1:%d/mcp" % target_port
        direct_mcp = "http://127.0.0.1:%d/mcp" % direct_port
        ingress_mcp = "http://127.0.0.1:%d/mcp" % ingress_port
        wait_for("target adhd", lambda: rpc(target_mcp, "tools/list")[1] == 200)
        wait_for("caller adhd (direct)", lambda: rpc(direct_mcp, "tools/list")[1] == 200)
        wait_for("caller adhd (ingress)", lambda: rpc(ingress_mcp, "tools/list")[1] == 200)

        border_port, iport, pport = free_port(), free_port(), free_port()
        border_addr = "127.0.0.1:%d" % border_port
        spawn([
            sys.executable, os.path.join(REPO, "sheets", "ingress", "border_adapter.py"),
            "--addr", border_addr, "--key", BORDER_KEY, "--target", target_mcp,
            "--connection-mode", args.connection_mode,
        ], "border")
        wait_for("signed border", lambda: tcp_open(border_port))

        agreements = {
            "agreements": [{
                "id": "workspace-42",
                "namespace": NAMESPACE,
                "border": "http://%s/cluster" % border_addr,
                "aliases": [AGREEMENT_ALIAS],
                "rung": 2,
            }]
        }
        agreements_path = os.path.join(tmp, "agreements.json")
        with open(agreements_path, "w") as f:
            json.dump(agreements, f)
        config = {
            "addr": "127.0.0.1:%d" % iport,
            "agreements": agreements_path,
            "token": TOKEN,
            "border_key": BORDER_KEY,
            "smoke_alarm": registry_url,
        }
        config_path = os.path.join(tmp, "ingress.json")
        with open(config_path, "w") as f:
            json.dump(config, f)

        spawn([os.path.join(tmp, "wan-ingress"), "-config", config_path], "ingress")
        spawn([
            os.path.join(tmp, "invite-proxy"),
            "-addr=127.0.0.1:%d" % pport,
            "-registry=" + registry_url,
            "-name=wan-ingress",
            "-token=" + TOKEN,
        ], "invite-proxy")
        invite_url = "http://127.0.0.1:%d" % pport
        wait_for("registered ingress", lambda: get(invite_url + "/healthz")[0] == 200)

        forward_url = invite_url + "/forward?name=" + AGREEMENT_ALIAS
        print("   direct:  adhd(%s) -> target" % direct_name)
        print("   ingress: adhd(%s) -> invite(wan-ingress) -> ingress -> signed border -> target" % ingress_name)

        print("\n## blue trust checks (prediction: boundary enforced once, not special-cased)")
        _, unsigned_status, _ = post(
            "http://%s/cluster/namespace/%s/name/%s" % (border_addr, NAMESPACE, AGREEMENT_ALIAS),
            {"jsonrpc": "2.0", "id": 1, "method": "adhd.status"},
        )
        _, untrusted_status, untrusted_body = post(
            "http://127.0.0.1:%d/forward?name=%s" % (iport, AGREEMENT_ALIAS),
            {"jsonrpc": "2.0", "id": 1, "method": "adhd.status"},
        )
        _, unknown_status, unknown_body = post(
            invite_url + "/forward?name=not-agreed",
            {"jsonrpc": "2.0", "id": 1, "method": "adhd.status"},
        )
        trust = {
            "unsigned_border_rejected": unsigned_status == 403,
            "caller_without_token_rejected": untrusted_status == 403,
            "unagreed_name_quarantined": (unknown_status == 422 and "unknown" in unknown_body),
        }
        for label, ok in trust.items():
            print("   %-32s %s" % (label, "PASS" if ok else "FAIL"))

        # Count only the measured path, not the one-off trust probes.
        get("http://%s/stats/reset" % border_addr)

        print("\n## baseline metric — %d calls per path, method adhd.status" % args.calls)
        adhd_proxy_call(direct_mcp, target_mcp, "adhd.status", 0)
        adhd_proxy_call(ingress_mcp, forward_url, "adhd.status", 0)

        direct_ms, ingress_ms = [], []
        direct_ok = ingress_ok = True
        first_direct_failure = first_ingress_failure = None
        for i in range(1, args.calls + 1):
            dt, status, body = adhd_proxy_call(direct_mcp, target_mcp, "adhd.status", i)
            direct_ms.append(dt)
            wrapped = proxy_result(body)
            if status != 200 or "error" in wrapped or "result" not in wrapped:
                direct_ok = False
                if first_direct_failure is None:
                    first_direct_failure = (status, body)

            dt, status, body = adhd_proxy_call(ingress_mcp, forward_url, "adhd.status", i)
            ingress_ms.append(dt)
            wrapped = proxy_result(body)
            # A JSON-RPC target is wrapped once by adhd.proxy (result.result);
            # the ingress's decision envelope is already the tool result.
            decision = wrapped.get("result") if "result" in wrapped else wrapped
            if (status != 200 or "error" in wrapped
                    or decision.get("decision") != "forward"
                    or decision.get("upstream_status") != 200):
                ingress_ok = False
                if first_ingress_failure is None:
                    first_ingress_failure = (status, body)

        border_stats_status, border_stats_body = get("http://%s/stats" % border_addr)
        border_stats = json.loads(border_stats_body) if border_stats_status == 200 else {"error": "stats unavailable"}

        baseline = log_latencies(os.path.expanduser("~/Library/Logs/kb-adhd-%s.jsonl" % direct_name), "tools/call")
        live = log_latencies(os.path.expanduser("~/Library/Logs/kb-adhd-%s.jsonl" % ingress_name), "tools/call")
        mod = load_blue_latency()
        rank = verdict(mod, baseline or direct_ms, live or ingress_ms)

        pct = mod.pct
        result = {
            "claim": "boundary is trust, not speed; both callers are the same adhd generation",
            "method": "adhd.status",
            "calls": args.calls,
            "paths": {
                "direct": {"target": target_mcp, "ok": direct_ok,
                           "n": len(direct_ms), "p50_ms": round(pct(direct_ms, 0.5), 3),
                           "p95_ms": round(pct(direct_ms, 0.95), 3)},
                "ingress": {"target": forward_url, "ok": ingress_ok,
                            "n": len(ingress_ms), "p50_ms": round(pct(ingress_ms, 0.5), 3),
                            "p95_ms": round(pct(ingress_ms, 0.95), 3)},
            },
            "overhead_median_ms": round(pct(ingress_ms, 0.5) - pct(direct_ms, 0.5), 3),
            "connection_mode": args.connection_mode,
            "border_stats": border_stats,
            "trust": trust,
            "blue_latency": rank,
            "note": "latency is the cost of the boundary, not a finding; the finding is unsigned-forward rejection",
        }
        with open(args.out, "w") as f:
            json.dump(result, f, indent=2)

        print("   direct   p50 %.2fms p95 %.2fms ok=%s" % (
            result["paths"]["direct"]["p50_ms"], result["paths"]["direct"]["p95_ms"], direct_ok))
        print("   ingress  p50 %.2fms p95 %.2fms ok=%s" % (
            result["paths"]["ingress"]["p50_ms"], result["paths"]["ingress"]["p95_ms"], ingress_ok))
        print("   overhead +%.2fms  (cost, not a finding)" % result["overhead_median_ms"])
        print("   connections (%s): %s" % (args.connection_mode, border_stats))
        print("   blue-latency: p=%.6f  %s" % (rank["p_value"], rank["label"]))
        if first_direct_failure:
            print("   first direct failure: %s" % (first_direct_failure,))
        if first_ingress_failure:
            print("   first ingress failure: %s" % (first_ingress_failure,))
        print("   result: %s" % args.out)

        if not (direct_ok and ingress_ok and all(trust.values())):
            raise RuntimeError("blue test failed: see result JSON")
        return 0
    finally:
        for proc in procs:
            if proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
        down(ADHD_SH, direct_name)
        down(ADHD_SH, ingress_name)
        down(ADHD_SH, target_name)
        down(ALARM_SH, reg_name)
        # spin.sh leaves stopped instance bookkeeping behind; that is the harness's
        # contract, not this test's state.
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
