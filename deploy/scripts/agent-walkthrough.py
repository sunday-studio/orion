#!/usr/bin/env python3
"""Walk every Agent CLI board against the real binary.

For each screen on the design canvas it prints the board's text, runs the real command against a
real local Core and an enrolled agent, and waits so you can compare. Nothing is installed and no
root is needed; everything lives in /tmp/orion-walk.

    make agent-walkthrough                 all screens
    make agent-walkthrough ONLY=status     only screens whose board or name contains "status"

Core runs on http://127.0.0.1:8999 with authentication off, so there is no login. To see the
Console too, run `make console-dev` in another terminal and open http://localhost:5173.
Screens that need a real server (service manager, root, a release) are shown and skipped.
"""
import json, os, re, shlex, shutil, signal, subprocess, sys, time, urllib.request

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
WORK = "/tmp/orion-walk"
PORT = os.environ.get("ORION_WALK_PORT", "8999")
CORE = f"http://127.0.0.1:{PORT}"
BOARDS = os.path.join(ROOT, "docs", "testing", "agent-cli-boards.json")
DIM, BOLD, RESET = "\033[2m", "\033[1m", "\033[0m"
HOST_CMD = re.compile(r"^orion-agent (start|stop|restart|update|service)\b|curl |\| bash|^… \| bash")
ENV = dict(os.environ, ORION_LOG_PATH=f"{WORK}/agent.log", EDITOR=os.environ.get("EDITOR", "vi"))


def sh(cmd, **kw):
    return subprocess.run(cmd, shell=True, env=kw.pop("env", ENV), **kw)


def agent(args, **kw):
    return subprocess.run([f"{WORK}/bin/orion-agent"] + args, env=kw.pop("env", ENV), **kw)


class Walk:
    core = None
    daemon = None

    # --- sandbox -----------------------------------------------------------------
    def build(self):
        os.makedirs(f"{WORK}/bin", exist_ok=True)
        print("Building Core and the agent (first time takes a minute)...")
        subprocess.run("CGO_ENABLED=1 go build -o %s/core ./cmd/api" % WORK, shell=True, cwd=f"{ROOT}/apps/core", check=True)
        subprocess.run("CGO_ENABLED=0 go build -o %s/oa ." % WORK, shell=True, cwd=f"{ROOT}/apps/agent", check=True)
        with open(f"{WORK}/bin/orion-agent", "w") as f:
            f.write("#!/bin/sh\nexec %s/oa --config %s/config.yaml --state %s/state.db --socket %s/a.sock \"$@\"\n" % (WORK, WORK, WORK, WORK))
        os.chmod(f"{WORK}/bin/orion-agent", 0o755)
        ENV["PATH"] = f"{WORK}/bin:" + ENV["PATH"]

    def healthy(self):
        try:
            urllib.request.urlopen(CORE + "/health", timeout=1)
            return True
        except Exception:
            return False

    def start_core(self):
        if self.core and self.core.poll() is None:
            return
        if self.healthy():
            sys.exit(f"Something else is already listening on port {PORT}. Stop it or set ORION_WALK_PORT.")
        env = dict(ENV, ORION_ALLOW_INSECURE_NO_AUTH="true", ORION_DATA_DIR=f"{WORK}/data", ORION_PORT=PORT)
        self.core = subprocess.Popen([f"{WORK}/core"], env=env, stdout=open(f"{WORK}/core.log", "w"), stderr=subprocess.STDOUT)
        for _ in range(60):
            if self.healthy():
                return
            time.sleep(0.25)
        sys.exit("Core did not start; see " + f"{WORK}/core.log")

    def stop_core(self):
        if self.core and self.core.poll() is None:
            self.core.terminate()
            self.core.wait()

    def stop_daemon(self):
        if self.daemon and self.daemon.poll() is None:
            self.daemon.terminate()
            try:
                self.daemon.wait(10)
            except subprocess.TimeoutExpired:
                self.daemon.kill()

    def reset(self):
        """A brand new host and an empty Core (Core knows hosts by machine id, so both go together)."""
        self.stop_daemon()
        self.stop_core()
        for p in ("data", "config.yaml", "state.db", "a.sock", "agent.log"):
            full = f"{WORK}/{p}"
            shutil.rmtree(full, ignore_errors=True) if os.path.isdir(full) else (os.path.exists(full) and os.remove(full))
        self.start_core()

    def registered(self):
        r = agent(["status", "--json"], capture_output=True, text=True)
        try:
            return bool(json.loads(r.stdout).get("registered"))
        except Exception:
            return False

    def enroll_quietly(self):
        if not os.path.exists(f"{WORK}/config.yaml"):
            agent(["setup", "--core-url", CORE, "--accept-discovered", "--yes", "--init-state"], capture_output=True)
        if not self.registered():
            agent(["enroll", "--core-url", CORE, "--token", "play"], capture_output=True)

    def start_daemon(self):
        self.daemon = subprocess.Popen([f"{WORK}/bin/orion-agent", "run"], env=ENV, stdout=open(f"{WORK}/run.log", "w"), stderr=subprocess.STDOUT)

    def wait_connected(self, seconds=45):
        for _ in range(seconds * 2):
            r = agent(["status", "--short"], capture_output=True, text=True)
            if "connected" in r.stdout:
                return True
            time.sleep(0.5)
        return False

    def ensure_running(self):
        self.start_core()
        self.enroll_quietly()
        if not (self.daemon and self.daemon.poll() is None):
            self.start_daemon()
        if not self.wait_connected():
            print("The agent did not connect; see", f"{WORK}/run.log")

    def cleanup(self):
        self.stop_daemon()
        self.stop_core()

    # --- running one command -----------------------------------------------------
    def run_command(self, cmd):
        cmd = re.sub(r"\s+\(or any pipe\)$", "", cmd).replace("https://core.example.com", CORE)
        cmd = cmd.replace("/usr/local/bin/backup.sh", "sh -c 'echo backup done'")
        print(f"\n{BOLD}$ {cmd}{RESET}")
        if cmd == "echo $?":
            return  # exit codes are printed after every command
        if HOST_CMD.search(cmd):
            print(f"{DIM}(needs a real server with a service manager or a release: skipped){RESET}")
            return
        if cmd == "orion-agent run":
            self.stop_daemon()
            print(f"{DIM}(foreground run: Ctrl+C when you have seen enough){RESET}")
        sys.stdout.flush()
        r = subprocess.run(cmd, shell=True, env=ENV, preexec_fn=lambda: signal.signal(signal.SIGINT, signal.SIG_DFL))
        print(f"{DIM}[exit {r.returncode}]{RESET}")

    # --- special screens ---------------------------------------------------------
    def screen_first_start(self):
        self.reset()
        self.enroll_quietly()
        self.start_daemon()
        for cmd in ("orion-agent status", "orion-agent status"):
            self.run_command(cmd)
            time.sleep(0.3)
        self.wait_connected()

    def screen_states(self):
        self.ensure_running()
        print(f"\n{DIM}Not running:{RESET}")
        self.stop_daemon()
        self.run_command("orion-agent status")
        print(f"\n{DIM}Waiting (right after the agent starts):{RESET}")
        self.start_daemon()
        self.run_command("orion-agent status")
        self.wait_connected()
        print(f"\n{DIM}Maintenance:{RESET}")
        self.run_command('orion-agent maintenance down "kernel upgrade" --for 2h')
        self.run_command("orion-agent status")
        self.run_command("orion-agent maintenance up")
        print(f"\n{DIM}\"Not enrolled\" and \"No recent report\" need a Core with authentication on or an 11 minute wait: host only.{RESET}")

    def screen_with_config(self, extra_yaml_by_cmd):
        path = f"{WORK}/config.yaml"
        backup = open(path).read()
        try:
            for cmd, extra in extra_yaml_by_cmd:
                open(path, "w").write(backup.rstrip("\n") + "\n" + extra)
                self.run_command(cmd)
        finally:
            open(path, "w").write(backup)

    # --- one screen --------------------------------------------------------------
    def run_screen(self, s):
        board, name, cmds = s["board"], s["screen"], s["commands"]
        key = (board, name)
        if board == "CLI, setup" and name != "non-interactive":
            return "setup-group"
        if board == "CLI, install script":
            return "host"
        if board == "CLI, log viewer":
            return "needs log shipping with an ingest key from the Console: host only"
        if board == "CLI, update and rollback" or (board.startswith("CLI, start, stop") and name != "run"):
            return "host"
        fresh = board == "CLI, setup" or name == "enroll" or board == "CLI, output modes and exit codes" and name == "accessible prompts"
        if name.startswith("first start"):
            self.screen_first_start()
            return None
        if name.endswith("the six states"):
            self.screen_states()
            return None
        if fresh:
            self.reset()
            if name == "enroll":
                print(f"{DIM}Token to type when asked: play{RESET}")
        elif name.startswith("agent offline"):
            self.ensure_running()
            self.stop_core()
        else:
            self.ensure_running()
        if name.startswith("scanning off"):
            self.screen_with_config([(cmds[0], "discovery:\n  enabled: false\n"), (cmds[1], "discovery:\n  kinds: [services, ports]\n")])
            return None
        if name.startswith("doctor") and "failing" in name:
            ENV["ORION_CORE_URL"] = "http://127.0.0.1:9"
            try:
                for c in cmds:
                    self.run_command(c)
            finally:
                ENV.pop("ORION_CORE_URL", None)
            return None
        for c in cmds:
            self.run_command(c)
        if name.startswith("agent offline"):
            self.start_core()
        return None


def main():
    sys.stdout.reconfigure(line_buffering=True)
    boards = json.load(open(BOARDS))
    only = os.environ.get("ONLY", "").lower().strip()
    walk = Walk()
    if not os.path.isdir(f"{ROOT}/apps/agent"):
        sys.exit("Run this from the repository.")
    walk.build()
    print(f"\nCore: {CORE} (no login). Console: run `make console-dev` in another terminal, open http://localhost:5173")
    walk.start_core()
    try:
        i = 0
        while i < len(boards):
            s = boards[i]
            i += 1
            if only and only not in (s["board"] + " " + s["screen"]).lower():
                continue
            group = [s]
            if s["board"] == "CLI, setup" and s["screen"] != "non-interactive":  # the interactive flow is one run
                while i < len(boards) and boards[i]["board"] == "CLI, setup" and boards[i]["screen"] != "non-interactive":
                    group.append(boards[i])
                    i += 1
                s = dict(s, screen="setup flow", commands=[])
            print("\n" + "=" * 90)
            print(f"{BOLD}{s['board']} / {' + '.join(g['screen'] for g in group)}{RESET}\n")
            for g in group:
                print(f"{DIM}BOARD ({g['screen']}):{RESET}\n{g['expected']}\n")
            if len(group) == 1:
                ans = input(f"{BOLD}Enter = run it, s = skip, q = quit > {RESET}").strip().lower()
                if ans == "q":
                    break
                skip_reason = "skipped" if ans == "s" else walk.run_screen(s)
            else:
                skip_reason = None
            if len(group) > 1:
                ans = input(f"{BOLD}Enter = run the interactive setup (token: play), s = skip, q = quit > {RESET}").strip().lower()
                if ans == "q":
                    break
                if ans != "s":
                    walk.reset()
                    sh("orion-agent setup", preexec_fn=lambda: signal.signal(signal.SIGINT, signal.SIG_DFL))
            elif skip_reason == "host":
                print(f"{DIM}Needs a real server (service manager, root or a release): shown above, not run here.{RESET}")
            elif skip_reason and skip_reason != "skipped":
                print(f"{DIM}{skip_reason}{RESET}")
            ans = input(f"\n{BOLD}Same as the board? Enter = next, q = quit > {RESET}").strip().lower()
            if ans == "q":
                break
    except KeyboardInterrupt:
        print()
    finally:
        walk.cleanup()
        print("Stopped Core and the agent. (Sandbox files stay in", WORK + ")")


if __name__ == "__main__":
    main()
