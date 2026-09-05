#!/usr/bin/env python3
# PreToolUse hook (Bash): when the command is a `git commit`, scan the staged
# diff's ADDED prose lines and the commit message for two shapes of claim that
# AGENTS.md's "Claims in authored prose" bans, and ASK when any is found.
#
#   1. An unnamed quantifier: nothing / never / always / every / only / none /
#      the one. Each asserts a search result. Either the line names the set or
#      the word goes. Text inside backticks is literal content and is skipped.
#   2. An unresolved pointer: a `dir/file.ext` or `file.ext:123` token in a
#      comment that names no file under the repo, the cwd, or the file's own
#      directory. A pointer is only worth more than a paraphrase while it
#      resolves.
#
# Prose means a line in a markdown or text file, or a line whose first
# non-space characters are the comment marker for the file's extension. Code
# lines are not scanned. The commit message is scanned for quantifiers when it
# is in the command (`-m` arguments or a `-F -` heredoc body).
#
# The decision is ASK, never deny. AGENTS.md names four carve-outs (an
# instruction, a quantifier about the function in front of you, a rhetorical
# aside about people, literal content), and only a reader can tell a carve-out
# from a claim. The reason lists every hit as file:line so the model can amend
# or approve each one on purpose. Zero hits emits nothing and the command falls
# through to the normal permission flow. Any failure (no git, no repo, a diff
# that will not parse) is silent, because this hook is a prompt for attention
# and never a gate the work depends on.
#
# Measured basis: one review-and-fix run wrote four wrong `only` claims with the
# same scan in place as a reading instruction, and another spent two iterations
# on a pointer that named the wrong paragraph.

import json
import os
import re
import shlex
import subprocess
import sys

KNOWN_GIT_PATHS = {
    "git",
    "/usr/bin/git",
    "/opt/homebrew/bin/git",
    "/usr/local/bin/git",
    "/opt/local/bin/git",
}
# Top-level git options that take a separate value, so the subcommand is the
# token after the value rather than the value itself.
VALUE_OPTS = {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix"}

QUANTIFIER = re.compile(r"\b(nothing|never|always|every|only|none|the one)\b", re.I)
BACKTICKS = re.compile(r"`[^`]*`")

PROSE_EXTS = {".md", ".mdx", ".txt", ".rst", ".adoc"}
COMMENT_MARKERS = {
    "#": {".py", ".sh", ".bash", ".zsh", ".rb", ".yml", ".yaml", ".toml", ".jq", ".pl", ".r"},
    "//": {".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".go", ".java", ".kt", ".kts",
           ".swift", ".rs", ".c", ".h", ".cc", ".cpp", ".hpp", ".cs", ".scala", ".php",
           ".dart", ".m"},
    "--": {".sql", ".lua", ".hs"},
}
# `/* ... */` and the leading `*` of a continued block comment, for the `//`
# family. `*` alone is also a markdown bullet, but markdown is scanned whole.
BLOCK_MARKERS = ("/*", "*")

SOURCE_EXTS = (
    "ts|tsx|js|jsx|mjs|cjs|py|go|rb|java|kt|rs|swift|c|h|cpp|cs|php|sh|md|json|yml|yaml|"
    "sql|toml|jq|txt|html|css|scss|vue|svelte"
)
# `a/b.ts`, `a/b.ts:12`, `b.ts:12`, `~/x/a.md`, `/abs/a.md`. A bare `b.ts` is too common a word shape
# (`package.json`) to treat as a pointer, so it needs a slash or a line number.
POINTER = re.compile(
    r"(?<![\w/@.~-])((?:~?/)?(?:[\w.-]+/)+[\w.-]+\.(?:%s)|[\w.-]+\.(?:%s):\d+)(?::\d+)?(?![\w/])"
    % (SOURCE_EXTS, SOURCE_EXTS)
)

MAX_HITS = 10


def _tokens(cmd):
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        return list(lex)
    except ValueError:
        return []


def _is_git_commit(tokens):
    # Find `git ... commit` at a command position: start, or after ; && || |.
    starts = [0] + [i + 1 for i, t in enumerate(tokens) if t in {";", "&&", "||", "|"}]
    for s in starts:
        if s >= len(tokens) or tokens[s] not in KNOWN_GIT_PATHS:
            continue
        i = s + 1
        while i < len(tokens):
            a = tokens[i]
            if not a.startswith("-"):
                if a == "commit":
                    return True
                break
            i += 2 if a in VALUE_OPTS else 1
    return False


def _message_text(cmd, tokens):
    # `-m msg` (repeatable) plus a `-F -` heredoc body when the command carries one.
    parts = []
    for i, t in enumerate(tokens):
        if t in {"-m", "--message"} and i + 1 < len(tokens):
            parts.append(tokens[i + 1])
        elif t.startswith("-m") and len(t) > 2 and not t.startswith("--"):
            parts.append(t[2:])
        elif t.startswith("--message="):
            parts.append(t[len("--message="):])
    m = re.search(r"<<-?\s*['\"]?(\w+)['\"]?[^\n]*\n(.*?)\n\1\s*$", cmd, re.S | re.M)
    if m:
        parts.append(m.group(2))
    return "\n".join(parts)


def _staged_diff(cwd):
    try:
        out = subprocess.run(
            ["git", "diff", "--cached", "-U0", "--no-color", "--no-ext-diff"],
            cwd=cwd, capture_output=True, text=True, timeout=20,
        )
    except Exception:
        return ""
    return out.stdout if out.returncode == 0 else ""


def _toplevel(cwd):
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            cwd=cwd, capture_output=True, text=True, timeout=10,
        )
    except Exception:
        return None
    return out.stdout.strip() if out.returncode == 0 else None


def _added_lines(diff):
    # Yields (path, new_line_number, text) for every added line.
    path = None
    new_ln = 0
    for raw in diff.splitlines():
        if raw.startswith("+++ "):
            p = raw[4:].strip()
            path = None if p == "/dev/null" else re.sub(r"^b/", "", p)
            continue
        if raw.startswith("--- ") or raw.startswith("diff ") or raw.startswith("index "):
            continue
        m = re.match(r"^@@ -\S+ \+(\d+)(?:,\d+)? @@", raw)
        if m:
            new_ln = int(m.group(1))
            continue
        if raw.startswith("+") and path is not None:
            yield path, new_ln, raw[1:]
            new_ln += 1
        elif raw.startswith(" "):
            new_ln += 1


def _is_prose(path, text):
    ext = os.path.splitext(path)[1].lower()
    if ext in PROSE_EXTS:
        return True
    s = text.lstrip()
    for marker, exts in COMMENT_MARKERS.items():
        if ext in exts:
            if s.startswith(marker):
                return True
            if marker == "//" and s.startswith(BLOCK_MARKERS):
                return True
            return False
    return False


def _quantifier_hits(text):
    stripped = BACKTICKS.sub("", text)
    if "://" in stripped:
        stripped = re.sub(r"\S+://\S+", "", stripped)
    return [m.group(1) for m in QUANTIFIER.finditer(stripped)]


def _pointer_hits(text, path, roots):
    # A pointer inside backticks is still a pointer. It has to resolve.
    hits = []
    stripped = re.sub(r"\S+://\S+", "", text)
    for m in POINTER.finditer(stripped):
        tok = m.group(0)
        target = re.sub(r":\d+$", "", m.group(1))
        cands = []
        if target.startswith("~"):
            cands.append(os.path.expanduser(target))
        elif os.path.isabs(target):
            cands.append(target)
        else:
            for r in roots:
                cands.append(os.path.join(r, target))
            cands.append(os.path.join(roots[0], os.path.dirname(path), target))
        if not any(os.path.exists(c) for c in cands):
            hits.append(tok)
    return hits


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    tokens = _tokens(cmd)
    if not tokens or not _is_git_commit(tokens):
        return
    cwd = data.get("cwd") or os.getcwd()
    top = _toplevel(cwd)
    if not top:
        return
    roots = [top, cwd]

    quant, ptrs = [], []
    for path, ln, text in _added_lines(_staged_diff(cwd)):
        if not _is_prose(path, text):
            continue
        for w in _quantifier_hits(text):
            quant.append(f"{path}:{ln}: `{w}` in: {text.strip()[:120]}")
        for p in _pointer_hits(text, path, roots):
            ptrs.append(f"{path}:{ln}: `{p}` does not resolve")

    for i, line in enumerate(_message_text(cmd, tokens).splitlines(), 1):
        for w in _quantifier_hits(line):
            quant.append(f"commit message line {i}: `{w}` in: {line.strip()[:120]}")

    if not quant and not ptrs:
        return

    hits = quant + ptrs
    shown = hits[:MAX_HITS]
    more = len(hits) - len(shown)
    reason = (
        "Staged prose or the commit message carries "
        f"{len(quant)} unnamed quantifier(s) and {len(ptrs)} unresolved pointer(s) "
        "(AGENTS.md, Claims in authored prose):\n  "
        + "\n  ".join(shown)
        + (f"\n  ... and {more} more" if more > 0 else "")
        + "\nFor each: name the set or drop the word, fix the pointer, then re-stage and commit. "
        "Approve only if every hit is a carve-out (an instruction, a claim about the function in "
        "front of you, an aside about people, or literal content)."
    )
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "ask",
                "permissionDecisionReason": reason,
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
