"""How often a candidate pattern fires on real commits, before anyone builds it.

Runs each candidate over the added lines of the last N non-merge commits on a
repo's default branch and prints, per candidate, how many commits it fires on
and sample lines. A human commit that trips a candidate is a false positive
unless the sample shows the rule really was broken, so read the samples.

    python3 scan.py ~/Dev/repos/github.com/calm/api [N=300]

Add a candidate by adding an entry to CANDIDATES: a name, a regex over one added
line with string literals blanked, and which files it reads ("prod", "test" or
"all"). Checks that need more than one line, such as the dead-reference and
vacuous-test ones, are measured with replay.py against the hook itself.
"""
import collections
import re
import subprocess
import sys

REPO = sys.argv[1] if len(sys.argv) > 1 else "."
N = int(sys.argv[2]) if len(sys.argv) > 2 else 300

TEST = re.compile(r"(\.test\.|\.spec\.|__tests__|/tests?/|/fixtures?/|test-utils|test-helpers)")
COMMENT = re.compile(r"^\s*(//|/\*|\*|#)")
STRINGS = re.compile(r"'(?:[^'\\\n]|\\.)*'|\"(?:[^\"\\\n]|\\.)*\"|`(?:[^`\\\n]|\\.)*`")

# (name, regex, scope, code_only)
CANDIDATES = [
    ("cast_as_type", re.compile(r"(?<![\w.])as\s+(?:any|[A-Z][\w.<>\[\]]*)\b(?!\s*from)"), "prod", True),
    ("double_cast", re.compile(r"\bas\s+unknown\s+as\b"), "prod", True),
    ("non_null_bang", re.compile(r"[\w\)\]]!(?=[.\[\),;\s]|$)(?!=)"), "prod", True),
    ("ticket_in_comment", re.compile(r"\b(?!UTC|GMT|ISO|SHA|UTF|RFC|TLS|CVE)[A-Z]{2,6}-\d{2,6}\b"), "all", False),
]
CODE_EXT = re.compile(r"\.(?:ts|tsx|js|jsx|mjs|cjs|py|go|rb|java|kt)$")


def git(*args):
    return subprocess.run(["git", *args], cwd=REPO, capture_output=True, text=True).stdout


def commits():
    out = git("log", "--no-merges", f"-{N}", "-p", "--unified=0", "--format=@@@COMMIT %h", "HEAD")
    cur, path = None, None
    for line in out.splitlines():
        if line.startswith("@@@COMMIT "):
            if cur:
                yield cur
            cur = {"sha": line.split()[1], "adds": []}
        elif line.startswith("+++ "):
            path = line[6:] if line.startswith("+++ b/") else None
        elif cur and path and line.startswith("+") and not line.startswith("+++"):
            cur["adds"].append((path, line[1:]))
    if cur:
        yield cur


def main():
    fired = collections.defaultdict(list)
    total = 0
    for c in commits():
        total += 1
        seen = set()
        for path, text in c["adds"]:
            if not CODE_EXT.search(path):
                continue
            is_test = bool(TEST.search(path))
            is_comment = bool(COMMENT.match(text))
            for name, rx, scope, code_only in CANDIDATES:
                if name in seen or (scope == "prod" and is_test) or (scope == "test" and not is_test):
                    continue
                if code_only and is_comment:
                    continue
                if (not code_only) and not is_comment:
                    continue
                if rx.search(STRINGS.sub("''", text) if code_only else text):
                    seen.add(name)
                    fired[name].append((c["sha"], f"{path}: {text.strip()[:110]}"))
    print(f"commits scanned: {total}")
    for name, _, _, _ in CANDIDATES:
        hits = fired[name]
        print(f"\n{name}: {len(hits)} commits ({100 * len(hits) / max(total, 1):.1f}%)")
        for sha, sample in hits[:6]:
            print(f"  {sha}  {sample}")


if __name__ == "__main__":
    main()
