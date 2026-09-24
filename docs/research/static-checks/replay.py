"""Replay check-staged-code.py's checks over past commits of a repo.

For each of the last N non-merge commits, the hook's index reads are pointed at
that commit's tree, so the result is what the hook would have said had the
commit been made through Claude. Use it after changing a check, to see the new
firing rate and the samples it fires on.

    python3 replay.py ~/Dev/repos/github.com/calm/api [N=300]
"""
import collections
import importlib.util
import os
import subprocess
import sys

REPO = sys.argv[1] if len(sys.argv) > 1 else "."
N = int(sys.argv[2]) if len(sys.argv) > 2 else 300
HOOK = os.path.expanduser("~/.melvin/config/shared_config/.claude/hooks/check-staged-code.py")

spec = importlib.util.spec_from_file_location("csc", HOOK)
csc = importlib.util.module_from_spec(spec)
spec.loader.exec_module(csc)


def git(*args):
    return subprocess.run(["git", *args], cwd=REPO, capture_output=True, text=True).stdout


def grep_at(sha):
    def run(_top, idents):
        if not idents:
            return []
        args = ["grep", "-n", "-w", "-F", "-I"]
        for i in sorted(idents):
            args += ["-e", i]
        # `git grep <sha>` prefixes each hit with "<sha>:", which the hook does not expect.
        return [ln.split(":", 1)[1] for ln in git(*args, sha).splitlines()]
    return run


def show_at(sha):
    return lambda _top, path: git("show", f"{sha}:{path}") or None


def main():
    shas = git("log", "--no-merges", f"-{N}", "--format=%h", "HEAD").split()
    fired = collections.defaultdict(list)
    for sha in shas:
        lines = list(csc._diff_lines(git("show", "-U0", "--no-color", "--format=", sha)))
        csc._git_grep = grep_at(sha)
        csc.pc._staged_file = show_at(sha)
        checks = {
            "dead_ref": lambda: csc._dead_refs(REPO, lines),
            "vacuous_test": lambda: csc._vacuous_tests(REPO, lines),
            "double_cast": lambda: csc._double_casts(lines),
        }
        for name, fn in checks.items():
            hits = fn()
            if hits:
                fired[name].append((sha, hits[0]))
    print(f"commits: {len(shas)}")
    for name in ("dead_ref", "vacuous_test", "double_cast"):
        hits = fired[name]
        print(f"\n{name}: {len(hits)} commits ({100 * len(hits) / max(len(shas), 1):.1f}%)")
        for sha, h in hits[:8]:
            print(f"  {sha} {h[:170]}")


if __name__ == "__main__":
    main()
