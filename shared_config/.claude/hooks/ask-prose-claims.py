#!/usr/bin/env python3
# PreToolUse hook (Bash): when the command is a `git commit`, scan the staged
# diff's ADDED prose lines and the commit message for two shapes of claim that
# AGENTS.md's "Claims in authored prose" bans, and DENY when any is found.
#
#   1. An unnamed quantifier: nothing / never / always / every / only / none /
#      the one. Each asserts a search result. Either the line names the set or
#      the word goes. Text inside backticks is literal content and is skipped.
#   2. An unresolved pointer: a `dir/file.ext` or `file.ext:123` token in a
#      comment that names no file under the repo, the cwd, or the file's own
#      directory. A pointer is only worth more than a paraphrase while it
#      resolves.
#   3. A comment in a code file naming an identifier (camelCase, snake_case,
#      or dotted) that no code line in the staged file names. That is a claim
#      about a collaborator, and thirteen of thirty self-inflicted fixes
#      across five review-and-fix runs were that shape. Case and underscores
#      are normalised so `payment_status` matches `paymentStatus`.
#   4. An added comment block in a code file longer than eight lines.
#
# Prose means a line in a markdown or text file, or a line whose first
# non-space characters are the comment marker for the file's extension. Code
# lines are not scanned. The commit message is scanned for quantifiers when it
# is in the command (`-m` arguments or a `-F -` heredoc body).
#
# The decision is DENY, with the hits as the reason. A hook's `ask` goes to the
# human, and the model never sees the reason unless the human declines, so an
# ask cannot make the model fix its own prose. A deny's reason is the tool
# result, so the model reads the file:line list, rewrites, and commits again.
#
# AGENTS.md names four carve-outs (an instruction, a quantifier about the
# function in front of you, a rhetorical aside about people, literal content),
# and only a reader can tell a carve-out from a claim. The override for those is
# a PROSE_CLAIMS_OK=1 prefix on the commit command. This hook lets that form
# through untouched. Because git runs outside the sandbox and that variable is
# not in bash-allow-trusted's safe_assignments, the prefixed command falls to
# the normal permission prompt, so the override lands in front of the human
# with the model's reason beside it. Claims go to the model, carve-outs go to
# the human, and neither is silent.
#
# Zero hits emits nothing and the command falls through to the normal
# permission flow. Any failure (no git, no repo, a diff that will not parse) is
# silent, because a broken check must not block every commit.
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
# A line starting with `*` is a block-comment continuation when what follows
# reads as prose, and a multiplication continuation when it reads as code.
# `* 2` and `* scaleFactor;` are code. `* Returns the row.` and `*/` are not.
STAR_CODE = re.compile(r"^\*\s*(?:[\d(\[{'\"`-]|[A-Za-z_$][\w$.]*\s*[;,)\]}]?\s*$)")
# Runtime globals and hook-style names a comment may mention without the file
# defining them. Names here are matched on the first segment.
BUILTIN_HEADS = {"json", "promise", "console", "math", "object", "array", "number", "string",
                 "date", "process", "window", "document", "react", "buffer", "symbol", "reflect",
                 "error", "map", "set", "regexp", "intl", "globalthis", "sinon", "jest", "expect",
                 "vi", "cy", "z", "t"}

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


ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")


def _is_git_commit(cmd):
    # Find `git ... commit` at a command position: start of a line, or after
    # ; && || | on that line. Lines are checked one by one because a newline is
    # a separator too, and the common shape is a heredoc or an echo on one line
    # and the commit on the next. A heredoc body line that begins with `git
    # commit` reads as a commit here, which denies rather than misses. Leading
    # VAR=value assignments are skipped, since `GIT_AUTHOR_NAME=x git commit`
    # is a commit.
    for line in cmd.splitlines():
        tokens = _tokens(line)
        starts = [0] + [i + 1 for i, t in enumerate(tokens) if t in {";", "&&", "||", "|"}]
        for s in starts:
            while s < len(tokens) and ASSIGN.match(tokens[s]) and tokens[s] not in KNOWN_GIT_PATHS:
                s += 1
            if s >= len(tokens) or tokens[s] not in KNOWN_GIT_PATHS:
                continue
            if _subcommand_is_commit(tokens, s + 1):
                return True
    return False


def _subcommand_is_commit(tokens, i):
    while i < len(tokens):
        a = tokens[i]
        if not a.startswith("-"):
            return a == "commit"
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
        elif re.fullmatch(r"-[a-zA-Z]*m", t) and i + 1 < len(tokens):
            # A bundled short cluster ending in m, `-am` or `-qm`, takes the next token.
            parts.append(tokens[i + 1])
        elif t.startswith("--message="):
            parts.append(t[len("--message="):])
    # Only a heredoc whose opening line is the commit command is the message. A
    # command can carry other heredocs, such as a python edit before the commit,
    # and those are code, not prose.
    for m in re.finditer(r"^([^\n]*)<<-?\s*['\"]?(\w+)['\"]?[^\n]*\n(.*?)\n\2\s*$", cmd, re.S | re.M):
        if _is_git_commit(m.group(1)):
            parts.append(m.group(3))
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
    prev = ""
    for raw in diff.splitlines():
        # A `+++ ` line is a file header only right after its `--- ` line. Inside a
        # hunk it is an added line whose content begins with `++ `, which a file
        # holding diff text produces, and reading it as a header would reset the
        # path for the rest of the hunk.
        if raw.startswith("+++ ") and prev.startswith("--- "):
            p = raw[4:].strip()
            path = None if p == "/dev/null" else re.sub(r"^b/", "", p)
            prev = raw
            continue
        prev = raw
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
            if marker == "//" and s.startswith("/*"):
                return True
            if marker == "//" and s.startswith("*") and not STAR_CODE.match(s):
                return True
            return False
    return False


def _quantifier_hits(text):
    stripped = BACKTICKS.sub("", text)
    if "://" in stripped:
        stripped = re.sub(r"\S+://\S+", "", stripped)
    return [m.group(1) for m in QUANTIFIER.finditer(stripped)]


# AGENTS.md "Plain ASCII in authored prose": each glyph and what to type instead.
UNICODE_SUBS = {
    "→": "->", "←": "<-", "…": "...", "≥": ">=", "≤": "<=",
    "×": "x", "—": "two sentences", "–": "two sentences",
    "“": '"', "”": '"', "‘": "'", "’": "'",
}


def _unicode_hits(text):
    stripped = BACKTICKS.sub("", text)
    return [f"{ch} -> {UNICODE_SUBS[ch]}" for ch in dict.fromkeys(c for c in stripped if c in UNICODE_SUBS)]


def _pointer_hits(text, path, roots):
    # A pointer inside backticks is still a pointer. It has to resolve.
    hits = []
    stripped = re.sub(r"\S+://\S+", "", text)
    # A scheme-less web path such as example.com/docs/guide.md is not a file.
    stripped = re.sub(r"\b[\w-]+\.(?:com|org|net|io|dev|co|ai|app|edu|gov)/\S*", "", stripped)
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


# camelCase with an inner capital, snake_case with two or more segments, or a
# dotted member access. Plain words are not identifiers.
IDENT = re.compile(r"\b([A-Za-z_]\w*\.[a-z_]\w*|[a-z][a-z0-9]*(?:[A-Z][a-zA-Z0-9]*)+|[a-z][a-z0-9]*(?:_[a-z0-9]+)+)\b")
MAX_COMMENT_BLOCK = 8
# Dotted abbreviations that the identifier regex would otherwise read as
# member access.
NOT_IDENTS = {"e.g", "i.e", "et.al", "a.k.a", "vs.", "cf."}


def _norm(ident):
    return re.sub(r"[_.]", "", ident).lower()


def _staged_file(cwd, path):
    try:
        out = subprocess.run(["git", "show", f":{path}"], cwd=cwd, capture_output=True, text=True, timeout=10)
    except Exception:
        return None
    return out.stdout if out.returncode == 0 else None


def _code_idents(path, content):
    # Identifiers named by the file's code lines, normalised, so a comment's
    # `payment_status` matches the code's `paymentStatus`. Comment lines are
    # excluded, or a comment could vouch for another comment.
    out = set()
    in_block = False
    for line in content.splitlines():
        s = line.strip()
        if in_block:
            if "*/" in s:
                in_block = False
            continue
        if s.startswith("/*") and "*/" not in s:
            in_block = True
            continue
        if _is_prose(path, line):
            continue
        for m in IDENT.finditer(line):
            out.add(_norm(m.group(1)))
    return out


def _ident_hits(text, code_idents):
    # A comment naming a symbol the file's code never names is a claim about
    # a collaborator, the shape 13 of 30 self-inflicted fixes had.
    hits = []
    scan = BACKTICKS.sub(lambda mm: " " + mm.group(0)[1:-1] + " ", text)
    scan = re.sub(r"\S+://\S+", "", scan)
    scan = re.sub(r"\b[\w-]+\.(?:com|org|net|io|dev|co|ai|app|edu|gov)\b\S*", "", scan)
    for m in IDENT.finditer(scan):
        ident = m.group(1)
        if ident.lower() in NOT_IDENTS:
            continue
        head = re.split(r"[._]", ident)[0].lower()
        if head in BUILTIN_HEADS or re.match(r"^use[A-Z]", ident):
            continue
        # A file name is the pointer check's business, not this one's.
        if re.search(r"\.(md|ts|tsx|js|py|json|yml|yaml|sh|sql)$", ident):
            continue
        if _norm(ident) not in code_idents:
            hits.append(ident)
    return hits


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    tokens = _tokens(cmd)
    if not tokens or not _is_git_commit(cmd):
        return
    if any(t.startswith("PROSE_CLAIMS_OK=") for t in tokens):
        return
    cwd = data.get("cwd") or os.getcwd()
    top = _toplevel(cwd)
    if not top:
        return
    roots = [top, cwd]

    quant, ptrs, idents, blocks, glyphs = [], [], [], [], []
    code_cache = {}
    run_path, run_start, run_len = None, 0, 0

    def close_block():
        if run_len > MAX_COMMENT_BLOCK:
            blocks.append(f"{run_path}:{run_start}: added comment block of {run_len} lines (limit {MAX_COMMENT_BLOCK})")

    for path, ln, text in _added_lines(_staged_diff(cwd)):
        prose = _is_prose(path, text)
        is_code_file = os.path.splitext(path)[1].lower() not in PROSE_EXTS
        # Comment block length, code files only. Consecutive added comment
        # lines in one file form a block.
        if prose and is_code_file and text.strip():
            if path == run_path and ln == run_start + run_len:
                run_len += 1
            else:
                close_block()
                run_path, run_start, run_len = path, ln, 1
        else:
            close_block()
            run_path, run_start, run_len = None, 0, 0
        if not prose:
            continue
        for w in _quantifier_hits(text):
            quant.append(f"{path}:{ln}: `{w}` in: {text.strip()[:120]}")
        for p in _pointer_hits(text, path, roots):
            ptrs.append(f"{path}:{ln}: `{p}` does not resolve")
        for g in _unicode_hits(text):
            glyphs.append(f"{path}:{ln}: {g}")
        if is_code_file:
            if path not in code_cache:
                # Diff paths are repo-root-relative, so the index read runs from the root.
                content = _staged_file(roots[0], path)
                code_cache[path] = _code_idents(path, content) if content is not None else None
            if code_cache[path] is not None:
                for ident in _ident_hits(text, code_cache[path]):
                    idents.append(f"{path}:{ln}: `{ident}` is named by this comment and by no code line in the file")
    close_block()

    for i, line in enumerate(_message_text(cmd, tokens).splitlines(), 1):
        for w in _quantifier_hits(line):
            quant.append(f"commit message line {i}: `{w}` in: {line.strip()[:120]}")
        for g in _unicode_hits(line):
            glyphs.append(f"commit message line {i}: {g}")

    if not quant and not ptrs and not idents and not blocks and not glyphs:
        return

    hits = quant + ptrs + idents + blocks + glyphs
    shown = hits[:MAX_HITS]
    more = len(hits) - len(shown)
    reason = (
        "Staged prose or the commit message carries "
        f"{len(quant)} unnamed quantifier(s), {len(ptrs)} unresolved pointer(s), "
        f"{len(idents)} comment symbol(s) the file's code does not name, {len(blocks)} "
        f"over-long comment block(s), and {len(glyphs)} non-ASCII punctuation mark(s) "
        "(AGENTS.md, Claims in authored prose, Code comments and Plain ASCII):\n  "
        + "\n  ".join(shown)
        + (f"\n  ... and {more} more" if more > 0 else "")
        + "\nFor each: name the set or drop the word, fix the pointer, point at a symbol this file "
        "names or delete the claim, cut the block, or type the ASCII shown, then re-stage and commit. "
        "If every hit is a carve-out (an instruction, a claim about the function in front of you, "
        "an aside about people, or literal content), say which carve-out each one is and rerun the "
        "commit prefixed with PROSE_CLAIMS_OK=1, which puts the override in front of the user."
    )
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            }
        },
        sys.stdout,
    )


if __name__ == "__main__":
    main()
