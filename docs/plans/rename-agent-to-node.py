#!/usr/bin/env python3
"""Mechanical part of the agent -> node rename (docs/plans/rename-agent-to-node.md).

Run from the repository root:   python3 docs/plans/rename-agent-to-node.py [--dry-run]
It is deterministic: running it on the same commit always produces the same tree, so a reviewer can
re-run it and compare. Hand fixes (migration, installer, etc.) are separate commits.

It changes text in tracked files and moves files whose names contain "agent". It leaves alone
anything that is history or means something else (see SKIP_PREFIXES and PROTECT).
"""
import os, re, subprocess, sys

DRY = "--dry-run" in sys.argv
ONLY = next((a.split("=", 1)[1] for a in sys.argv if a.startswith("--only=")), "")  # e.g. --only=apps/console/
ROOT = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
os.chdir(ROOT)

# Paths never touched: history, design canvas, generated output, other owners' areas, AI-agent files.
SKIP_PREFIXES = (
    "docs/design/", "docs/milestones/", ".agents/", "docs/plans/", "apps/core/docs/",
    "apps/core/internal/db/migrations/0", "apps/console/", "apps/website/", "packages/",
)
# db.go holds the Go migration 000032, whose name is part of its stored checksum and which runs before
# the rename migration, so it must keep the old table and column names.
SKIP_EXACT = {"AGENTS.md", "COORDINATION.md", "skills-lock.json", "pnpm-lock.yaml", "apps/core/internal/db/db.go"}
# Planning docs that are live terms, not history.
INCLUDE_EXACT = {"docs/plans/coordinator-handoff.md"}
BINARY_EXT = (".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".otf", ".db", ".zip", ".gz", ".pdf")

# Words that look like "agent" but mean something else. Replaced by a placeholder, then restored.
PROTECT = [
    "User-Agent", "user-agent", "User agent", "user agent", "user_agent", "UserAgent", "useragent",
    "AI agents", "AI agent", "coding agents", "coding agent", "LLM agent", "agentic", "AGENTS.md", ".agents/",
]

def skipped(path):
    if ONLY:
        # Only this area, and its own skips do not apply (the Console is skipped by default).
        return not path.startswith(ONLY) or path.lower().endswith(BINARY_EXT)
    if path in INCLUDE_EXACT:
        return False
    return path in SKIP_EXACT or path.startswith(SKIP_PREFIXES) or path.lower().endswith(BINARY_EXT)

def rename_text(s):
    holds = {}
    for i, p in enumerate(PROTECT):
        key = f"\x00P{i}\x00"
        if p in s:
            s = s.replace(p, key); holds[key] = p
    for old, new in (("agents", "nodes"), ("Agents", "Nodes"), ("AGENTS", "NODES"),
                     ("agent", "node"), ("Agent", "Node"), ("AGENT", "NODE")):
        s = s.replace(old, new)
    s = re.sub(r"\ban (node|Node|NODE|nodes)\b", lambda m: "a " + m.group(1), s)
    s = re.sub(r"\bAn (node|Node|NODE)\b", lambda m: "A " + m.group(1), s)
    for key, p in holds.items():
        s = s.replace(key, p)
    return s

def rename_path(path):
    return path.replace("agents", "nodes").replace("agent", "node").replace("Agents", "Nodes").replace("Agent", "Node")

files = subprocess.check_output(["git", "ls-files", "-z"]).decode().split("\0")
files = [f for f in files if f and not skipped(f) and os.path.isfile(f)]
changed, moved = 0, []
for f in files:
    try:
        text = open(f, encoding="utf-8").read()
    except (UnicodeDecodeError, OSError):
        continue
    new = rename_text(text)
    if new != text:
        changed += 1
        if not DRY:
            open(f, "w", encoding="utf-8").write(new)
    target = rename_path(f)
    if target != f:
        moved.append((f, target))
print(f"{'would change' if DRY else 'changed'} {changed} files, {'would move' if DRY else 'moving'} {len(moved)} files")
if not DRY:
    for src, dst in sorted(moved):
        os.makedirs(os.path.dirname(dst) or ".", exist_ok=True)
        subprocess.check_call(["git", "mv", src, dst])
else:
    for src, dst in sorted(moved)[:25]:
        print("  ", src, "->", dst)
