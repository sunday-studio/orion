#!/usr/bin/env python3
"""Walk every Node CLI board against the real binary.

For each screen on the design canvas it prints the board's text, runs the real command against a
real local Core and an enrolled node, and waits so you can compare. Nothing is installed and no
root is needed; everything lives in /tmp/orion-walk.

    make node-walkthrough                 all screens
    make node-walkthrough ONLY=status     only screens whose board or name contains "status"

Core runs on http://127.0.0.1:8999. To see the Console too, run `make console-dev` in another
terminal, open http://localhost:5173 and log in with the username and password the script prints (new each run, local only).
Screens that need a real server (service manager, root, a release) are shown and skipped.
"""
import json, os, re, secrets, shlex, shutil, signal, subprocess, sys, time, urllib.request

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
WORK = os.environ.get("ORION_WALK_DIR") or f"/tmp/orion-walk-{os.getuid()}"  # short: unix socket paths are limited to ~100 bytes; set ORION_WALK_DIR to run a second copy side by side
PORT = os.environ.get("ORION_WALK_PORT", "8999")
CORE = f"http://127.0.0.1:{PORT}"
BOARDS = os.path.join(ROOT, "docs", "testing", "node-cli-boards.json")
DIM, BOLD, RESET = "\033[2m", "\033[1m", "\033[0m"
HOST_CMD = re.compile(r"^orion-node (start|stop|restart|update|service)\b|curl |\| bash|^… \| bash")
ADMIN_USER, ADMIN_PASSWORD = "admin", secrets.token_urlsafe(12)  # new every run, never stored
JWT_SECRET = secrets.token_hex(32)  # once per run, so a Console login survives the resets between screens
# Vite moves to the next port when 5173 is taken (another project often holds it), and Core only answers
# browser origins it knows, so allow the whole range the dev server can land on.
CORS_ORIGINS = ",".join(f"http://{h}:{p}" for p in range(5173, 5181) for h in ("localhost", "127.0.0.1"))
ENV = dict(os.environ, ORION_LOG_PATH=f"{WORK}/node.log", EDITOR=os.environ.get("EDITOR", "vi"))


def check_work_dir():
    """The sandbox dir must be ours and not a symlink, so another user cannot plant files in it."""
    if os.path.lexists(WORK):
        st = os.lstat(WORK)
        if os.path.islink(WORK) or st.st_uid != os.getuid():
            sys.exit(f"{WORK} exists but is not yours (or is a symlink): remove it and run again.")
    os.makedirs(WORK, mode=0o700, exist_ok=True)


class IgnoreInterrupt:
    """Ctrl+C belongs to the command being run (its child resets SIGINT); the walkthrough carries on."""
    def __enter__(self):
        self.old = signal.signal(signal.SIGINT, signal.SIG_IGN)

    def __exit__(self, *a):
        signal.signal(signal.SIGINT, self.old)


def child_default_sigint():
    signal.signal(signal.SIGINT, signal.SIG_DFL)


def sh(cmd, **kw):
    return subprocess.run(cmd, shell=True, env=kw.pop("env", ENV), **kw)


def node(args, **kw):
    return subprocess.run([f"{WORK}/bin/orion-node"] + args, env=kw.pop("env", ENV), **kw)


class Walk:
    core = None
    daemon = None

    # --- sandbox -----------------------------------------------------------------
    def build(self):
        check_work_dir()
        os.makedirs(f"{WORK}/bin", exist_ok=True)
        print("Building Core and the node (first time takes a minute)...")
        subprocess.run("CGO_ENABLED=1 go build -o %s/core ./cmd/api" % WORK, shell=True, cwd=f"{ROOT}/apps/core", check=True)
        subprocess.run("CGO_ENABLED=0 go build -o %s/oa ." % WORK, shell=True, cwd=f"{ROOT}/apps/node", check=True)
        with open(f"{WORK}/bin/orion-node", "w") as f:
            f.write("#!/bin/sh\nexec %s/oa --config %s/config.yaml --state %s/state.db --socket %s/a.sock \"$@\"\n" % (WORK, WORK, WORK, WORK))
        os.chmod(f"{WORK}/bin/orion-node", 0o755)
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
        env = dict(ENV, ORION_ADMIN_USERNAME=ADMIN_USER, ORION_ADMIN_PASSWORD=ADMIN_PASSWORD, ORION_JWT_SECRET=JWT_SECRET,
                   ORION_CORS_ORIGINS=CORS_ORIGINS, ORION_LISTEN_HOST="127.0.0.1", ORION_DATA_DIR=f"{WORK}/data", ORION_PORT=PORT)  # loopback only: Core with a login would otherwise bind every interface
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
        for p in ("data", "config.yaml", "state.db", "a.sock", "node.log"):
            full = f"{WORK}/{p}"
            shutil.rmtree(full, ignore_errors=True) if os.path.isdir(full) else (os.path.exists(full) and os.remove(full))
        self.start_core()

    def registered(self):
        r = node(["status", "--json"], capture_output=True, text=True)
        try:
            return bool(json.loads(r.stdout).get("registered"))
        except Exception:
            return False

    def enroll_quietly(self):
        if not os.path.exists(f"{WORK}/config.yaml"):
            node(["setup", "--core-url", CORE, "--accept-discovered", "--yes", "--init-state"], capture_output=True)
        if not self.registered():
            node(["enroll", "--core-url", CORE, "--token", "play"], capture_output=True)

    def start_daemon(self):
        self.daemon = subprocess.Popen([f"{WORK}/bin/orion-node", "run"], env=ENV, stdout=open(f"{WORK}/run.log", "w"), stderr=subprocess.STDOUT)

    def wait_connected(self, seconds=45):
        for _ in range(seconds * 2):
            r = node(["status", "--short"], capture_output=True, text=True)
            if "connected" in r.stdout or "maintenance" in r.stdout.lower():
                return True
            time.sleep(0.5)
        return False

    def ensure_running(self):
        self.start_core()
        self.enroll_quietly()
        if not (self.daemon and self.daemon.poll() is None):
            self.start_daemon()
        else:
            node(["maintenance", "up"], capture_output=True)  # one screen must not leave the next in maintenance
        if not self.wait_connected():
            print("The node did not connect; see", f"{WORK}/run.log")

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
        if cmd == "orion-node run":
            self.stop_daemon()
            print(f"{DIM}(foreground run: Ctrl+C when you have seen enough){RESET}")
        sys.stdout.flush()
        with IgnoreInterrupt():
            r = subprocess.run(cmd, shell=True, env=ENV, preexec_fn=child_default_sigint)
        print(f"{DIM}[exit {r.returncode}]{RESET}")

    def with_real_ids(self, cmds):
        """The board names ports from its own example host; use two suggestions that exist here, dismissing one first."""
        r = node(["discover", "--json"], capture_output=True, text=True)
        try:
            ids = [i["id"] for i in json.loads(r.stdout).get("items") or [] if not i.get("dismissed")]
        except Exception:
            ids = []
        if len(ids) < 2:
            return cmds
        node(["discover", "dismiss", ids[1]], capture_output=True)
        return [c.replace("port:5432", ids[0]).replace("port:8081", ids[1]) for c in cmds]

    # --- special screens ---------------------------------------------------------
    def screen_first_start(self):
        self.reset()
        self.enroll_quietly()
        self.start_daemon()
        for cmd in ("orion-node status", "orion-node status"):
            self.run_command(cmd)
            time.sleep(0.3)
        self.wait_connected()

    def screen_states(self):
        self.ensure_running()
        print(f"\n{DIM}Not running:{RESET}")
        self.stop_daemon()
        self.run_command("orion-node status")
        print(f"\n{DIM}Waiting (right after the node starts):{RESET}")
        self.start_daemon()
        self.run_command("orion-node status")
        self.wait_connected()
        print(f"\n{DIM}Maintenance:{RESET}")
        self.run_command('orion-node maintenance down "kernel upgrade" --for 2h')
        self.run_command("orion-node status")
        self.run_command("orion-node maintenance up")
        print(f"\n{DIM}\"Not enrolled\" and \"No recent report\" need a Core with authentication on or an 11 minute wait: host only.{RESET}")

    def screen_with_config(self, configs_by_cmd):
        """Run each command with the daemon started on a small config of its own, then put the real config back."""
        path = f"{WORK}/config.yaml"
        backup = open(path).read()
        try:
            for cmd, extra in configs_by_cmd:
                self.stop_daemon()
                open(path, "w").write(f"core_url: {CORE}\n{extra}")
                self.start_daemon()
                self.wait_connected()
                time.sleep(2)  # let the first scan finish
                self.run_command(cmd)
        finally:
            self.stop_daemon()
            open(path, "w").write(backup)

    # --- one screen --------------------------------------------------------------
    def run_screen(self, s):
        board, name, cmds = s["board"], s["screen"], s["commands"]
        key = (board, name)
        if board == "CLI, setup" and name != "non-interactive":
            return "setup-group"
        if board == "CLI, install script":
            return "host"
        if name.startswith("node offline"):
            return "\"offline since\" appears after several minutes without a report, so it is not reproduced here"
        if board == "CLI, log viewer":
            return "needs log shipping with an ingest key from the Console: host only"
        if name == "config edit" and not sys.stdin.isatty():
            return "opens your editor, so it needs a terminal"
        if board == "CLI, update and rollback" or (board.startswith("CLI, start, stop") and name != "run"):
            return "host"
        fresh = (board == "CLI, setup" and name != "non-interactive") or name == "enroll" or board == "CLI, output modes and exit codes" and name == "accessible prompts"
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
        if name.startswith("dismissed"):
            cmds = self.with_real_ids(cmds)
        for c in cmds:
            self.run_command(c)
        return None


def main():
    sys.stdout.reconfigure(line_buffering=True)
    boards = json.load(open(BOARDS))
    only = os.environ.get("ONLY", "").lower().strip()
    walk = Walk()
    if not os.path.isdir(f"{ROOT}/apps/node"):
        sys.exit("Run this from the repository.")
    walk.build()
    print(f"\nCore: {CORE}\nConsole: run `make console-dev` in another terminal, open the address it prints (usually http://localhost:5173) and log in:\n  username  {ADMIN_USER}\n  password  {ADMIN_PASSWORD}   (new every run)")
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
                    with IgnoreInterrupt():
                        sh("orion-node setup", preexec_fn=child_default_sigint)
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
        print("Stopped Core and the node. (Sandbox files stay in", WORK + ")")


if __name__ == "__main__":
    main()
