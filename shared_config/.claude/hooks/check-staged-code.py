#!/usr/bin/env python3
# PreToolUse hook (Bash): on `git commit`, read the staged diff and deny, with
# file:line reasons, when it carries one of three code shapes review keeps
# finding. Each is a pattern check, so each has a measured false-positive rate
# on human commits, in AGENTS.md "The commit-time code check".
#
#   dead reference  a symbol the diff removed that no code line in the index
#                   names any more, while a markdown file or a comment still does
#   vacuous test    an added it()/test() whose assertions are all absence checks,
#                   a bare .to.throw(), or a loose sinon.match
#   double cast     an added `as unknown as` in a non-test source file
#
# CODE_CHECKS_OK=1 in front of the commit lets it through, and because that
# variable is not in bash-allow-trusted's safe_assignments the prefixed command
# falls to the permission prompt. The commit detection and diff reading come
# from ask-prose-claims.py, loaded by path so the two hooks cannot drift. Any
# failure here is silent, because a broken check must not block every commit.

import importlib.util
import json
import os
import re
import subprocess
import sys

HOOK_DIR = os.path.dirname(os.path.realpath(__file__))
_spec = importlib.util.spec_from_file_location("ask_prose_claims", os.path.join(HOOK_DIR, "ask-prose-claims.py"))
pc = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(pc)

MAX_HITS = 10
CODE_EXTS = set().union(*pc.COMMENT_MARKERS.values())
JS_EXTS = {".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}
TEST_FILE = re.compile(r"\.(test|spec)\.[a-z]+$|_test\.[a-z]+$|(^|/)test_[^/]+\.py$")
TEST_DIR = re.compile(r"(^|/)(__tests__|tests?|fixtures?|__mocks__|test-utils|test-helpers)/")
STRINGS = re.compile(r"'(?:[^'\\\n]|\\.)*'|\"(?:[^\"\\\n]|\\.)*\"|`(?:[^`\\\n]|\\.)*`")

# camelCase or PascalCase with an inner capital, or snake_case of three or more
# segments. Shorter names collide with ordinary words in docs.
DEAD_IDENT = re.compile(r"\b([a-zA-Z][a-z0-9]+(?:[A-Z][a-z0-9]*)+|[a-z][a-z0-9]*(?:_[a-z0-9]+){2,})\b")
# A changelog and an architecture decision record describe the past on purpose.
DEAD_SKIP_PATHS = re.compile(r"(^|/)CHANGELOG[^/]*$|(^|/)(adr|adrs|decisions)/", re.I)

TEST_START = re.compile(r"^(\s*)(?:it|test)(?:\.only)?\s*\(")
ASSERTION = re.compile(r"\bexpect\s*\(|\bassert\.|\bsinon\.assert\.|\.should\.")
# Call absence, bare existence, a throw or rejection with no matcher, and a
# loose sinon matcher. A value that is undefined, empty or unequal is not here,
# because on master those were the behaviour the test was named for.
WEAK = re.compile(
    r"\.not\.(?:have\.)?been\.called\b|\.not\.to\.have\.been\.called\b|\bnotCalled\b"
    r"|\.called\)\.to\.(?:be\.)?false|callCount\)\.to\.(?:equal|eq)\(0\)|not\.toHaveBeenCalled\(\)"
    r"|\.to\.exist\b|\.to\.be\.ok\b|toBeDefined\(\)|toBeTruthy\(\)"
    r"|sinon\.match\.(?:any|string|number|object|func)\b|\.to\.throw\(\)|toThrow\(\)"
    r"|\.to\.be\.rejected\b(?!With)|rejectedWith\(\)"
)
DOUBLE_CAST = re.compile(r"\bas\s+unknown\s+as\b")


def _is_test(path):
    return bool(TEST_FILE.search(path) or TEST_DIR.search(path))


def _diff_lines(diff):
    # Yields (path, sign, new_line_number, text) for every added and removed
    # line. A removed line carries its old path and the number is 0.
    # `--- ` and `+++ ` are file headers only between `diff --git` and the first
    # `@@`. Inside a hunk they are content lines of a file that holds diff text.
    old = new = None
    ln = 0
    header = False
    for raw in diff.splitlines():
        if raw.startswith("diff --git "):
            header, old, new = True, None, None
            continue
        if header:
            if raw.startswith("--- "):
                p = raw[4:].strip()
                old = None if p == "/dev/null" else re.sub(r"^a/", "", p)
            elif raw.startswith("+++ "):
                p = raw[4:].strip()
                new = None if p == "/dev/null" else re.sub(r"^b/", "", p)
        m = re.match(r"^@@ -\S+ \+(\d+)(?:,\d+)? @@", raw)
        if m:
            header = False
            ln = int(m.group(1))
            continue
        if header:
            continue
        if raw.startswith("+") and new:
            yield new, "+", ln, raw[1:]
            ln += 1
        elif raw.startswith("-") and old:
            yield old, "-", 0, raw[1:]


def _git_grep(top, idents):
    if not idents:
        return []
    args = ["git", "grep", "--cached", "-n", "-w", "-F", "-I"]
    for i in sorted(idents):
        args += ["-e", i]
    try:
        out = subprocess.run(args, cwd=top, capture_output=True, text=True, timeout=20)
    except Exception:
        return []
    return out.stdout.splitlines() if out.returncode in (0, 1) else []


def _dead_refs(top, lines):
    added = "\n".join(t for _, s, _, t in lines if s == "+")
    cands = set()
    for path, sign, _, text in lines:
        ext = os.path.splitext(path)[1].lower()
        if sign != "-" or ext not in CODE_EXTS or _is_test(path) or pc._is_prose(path, text):
            continue
        for m in DEAD_IDENT.finditer(text):
            if not re.search(r"\b%s\b" % re.escape(m.group(1)), added):
                cands.add(m.group(1))
    if not cands:
        return []
    alive, docs = set(), {}
    for hit in _git_grep(top, cands):
        parts = hit.split(":", 2)
        if len(parts) < 3:
            continue
        path, ln, text = parts
        named = [c for c in cands if re.search(r"\b%s\b" % re.escape(c), text)]
        ext = os.path.splitext(path)[1].lower()
        is_doc = ext in pc.PROSE_EXTS or pc._is_prose(path, text)
        for c in named:
            if is_doc:
                if not DEAD_SKIP_PATHS.search(path):
                    docs.setdefault(c, []).append(f"{path}:{ln}")
            elif ext in CODE_EXTS:
                alive.add(c)
    out = []
    for c in sorted(docs):
        if c in alive:
            continue
        where = ", ".join(docs[c][:3]) + (f" and {len(docs[c]) - 3} more" if len(docs[c]) > 3 else "")
        out.append(f"`{c}` is gone from every code line in the index but still named at {where}")
    return out


def _vacuous_tests(top, lines):
    added = {}
    for path, sign, ln, _ in lines:
        if sign == "+" and os.path.splitext(path)[1].lower() in JS_EXTS and TEST_FILE.search(path):
            added.setdefault(path, set()).add(ln)
    out = []
    for path, lns in added.items():
        content = pc._staged_file(top, path)
        if content is None:
            continue
        rows = content.splitlines()
        for i, row in enumerate(rows, 1):
            m = TEST_START.match(row)
            if not m or i not in lns:
                continue
            indent = m.group(1)
            body = []
            for nxt in rows[i:]:
                if nxt.startswith(indent + "})") or (nxt.startswith(indent + "}") and nxt.strip().startswith("})")):
                    break
                body.append(nxt)
            asserts = [b for b in body if ASSERTION.search(b)]
            if asserts and all(WEAK.search(b) for b in asserts):
                name = re.search(r"\(\s*(['\"`])(.*?)\1", row)
                label = name.group(2)[:80] if name else row.strip()[:80]
                out.append(f"{path}:{i}: test \"{label}\" has {len(asserts)} assertion(s) and each is an absence check or a bare matcher")
    return out


def _double_casts(lines):
    out = []
    for path, sign, ln, text in lines:
        ext = os.path.splitext(path)[1].lower()
        if sign != "+" or ext not in JS_EXTS or _is_test(path) or pc._is_prose(path, text):
            continue
        if DOUBLE_CAST.search(STRINGS.sub("''", text)):
            out.append(f"{path}:{ln}: `as unknown as` in: {text.strip()[:110]}")
    return out


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    tokens = pc._tokens(cmd)
    if not tokens or not pc._is_git_commit(cmd):
        return
    if any(t.startswith("CODE_CHECKS_OK=") for t in tokens):
        return
    top = pc._toplevel(data.get("cwd") or os.getcwd())
    if not top:
        return
    lines = list(_diff_lines(pc._staged_diff(top)))
    if not lines:
        return

    dead = _dead_refs(top, lines)
    vacuous = _vacuous_tests(top, lines)
    casts = _double_casts(lines)
    hits = dead + vacuous + casts
    if not hits:
        return
    shown = hits[:MAX_HITS]
    more = len(hits) - len(shown)
    reason = (
        f"The staged diff carries {len(dead)} dead reference(s), {len(vacuous)} test(s) that cannot "
        f"fail for the right reason, and {len(casts)} `as unknown as` cast(s) in production code "
        "(AGENTS.md, The commit-time code check):\n  "
        + "\n  ".join(shown)
        + (f"\n  ... and {more} more" if more > 0 else "")
        + "\nFor each: update or delete the doc or comment that names the removed symbol, give the "
        "test a positive assertion that fails without the change, or replace the cast with a type "
        "guard or a validated parse. Then re-stage and commit. If a hit is intended, such as a doc "
        "that records history on purpose or a test whose whole point is that nothing is called, say "
        "why and rerun the commit prefixed with CODE_CHECKS_OK=1, which puts the override in front "
        "of the user."
    )
    json.dump(
        {"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny",
                                "permissionDecisionReason": reason}},
        sys.stdout,
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:
        pass
