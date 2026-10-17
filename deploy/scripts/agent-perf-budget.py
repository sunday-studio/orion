#!/usr/bin/env python3
"""Agent performance budgets (plan B4). Fails when a budget is exceeded.

  python3 deploy/scripts/agent-perf-budget.py path/to/orion-agent

CLI cold start: `version` and `--help` under 50 ms, `status` under 100 ms (median of 15).
Daemon: idle CPU under 1% and RSS under 30 MB with 20 monitors (measured over ~20 s)."""
import http.server, json, os, statistics, subprocess, sys, tempfile, threading, time

BIN = sys.argv[1] if len(sys.argv) > 1 else "orion-agent"
BUDGETS_MS = {"version": 50, "--help": 50, "status": 100}
RSS_MB, CPU_PCT = 30, 1.0
failed = False

def cold(args):
    runs = []
    for _ in range(15):
        t = time.perf_counter()
        subprocess.run([BIN] + args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        runs.append((time.perf_counter() - t) * 1000)
    return statistics.median(runs)

for name, limit in BUDGETS_MS.items():
    args = ["status", "--socket", "/nonexistent/x.sock"] if name == "status" else [name]
    ms = cold(args)
    ok = ms <= limit
    failed |= not ok
    print(f"{'ok  ' if ok else 'FAIL'} cold start {name}: {ms:.1f} ms (budget {limit} ms)")

class Core(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        data = {"agent_id": "a1", "token": "t1", "monitor_id": "m" + str(abs(hash(self.path)) % 10**6)}
        body = json.dumps({"success": True, "data": data}).encode()
        self.send_response(200); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
    do_PUT = do_POST
    def do_GET(self): self.do_POST()
    def log_message(self, *a): pass

srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Core)
port = srv.server_address[1]
threading.Thread(target=srv.serve_forever, daemon=True).start()

d = tempfile.mkdtemp()
mons = "".join(
    f"  - name: tcp-{i}\n    type: tcp\n    interval: 30s\n    tcp: {{host: 127.0.0.1, port: {port}, timeout: 2s}}\n" for i in range(20))
open(f"{d}/config.yaml", "w").write(f"core_url: http://127.0.0.1:{port}\ninterval: 30s\nlogging:\n  path: stdout\nmonitors:\n{mons}")
p = subprocess.Popen([BIN, "run", "--config", f"{d}/config.yaml", "--state", f"{d}/state.db", "--socket", f"{d}/s.sock"],
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(8)  # settle: registration and the first round of checks

def stats():
    out = subprocess.run(["ps", "-o", "rss=,cputime=", "-p", str(p.pid)], capture_output=True, text=True).stdout.split()
    rss_mb = int(out[0]) / 1024
    parts = out[1].replace("-", ":").split(":")
    secs = sum(float(x) * m for x, m in zip(reversed(parts), (1, 60, 3600, 86400)))
    return rss_mb, secs

rss0, cpu0 = stats(); t0 = time.time(); peak = rss0
while time.time() - t0 < 20:
    time.sleep(2); peak = max(peak, stats()[0])
rss1, cpu1 = stats()
cpu_pct = (cpu1 - cpu0) / (time.time() - t0) * 100
p.terminate(); p.wait(timeout=10)
for label, val, limit, unit in (("daemon RSS (peak)", peak, RSS_MB, "MB"), ("daemon idle CPU", cpu_pct, CPU_PCT, "%")):
    ok = val <= limit
    failed |= not ok
    print(f"{'ok  ' if ok else 'FAIL'} {label}: {val:.2f} {unit} (budget {limit} {unit})")
sys.exit(1 if failed else 0)
