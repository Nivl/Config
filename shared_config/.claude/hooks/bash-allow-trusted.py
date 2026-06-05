#!/usr/bin/env python3
# PreToolUse hook for the sandbox's EXCLUDED commands (they run
# unsandboxed, where permissions.allow does not suppress prompts).
#
# Data is DERIVED by `melvin-config claude perms` into a sibling
# bash-allow-trusted.json (committed) and may be augmented by a
# gitignored bash-allow-trusted.local.json. Both hold two lists of
# token-tuples: `excluded` (any sandbox-excluded command) and `trusted`
# (the allowed ∩ excluded subset). The hook UNIONs the two files.
#
# Decisions (first match wins):
#   1. Compound (| && || ; & newline) AND any segment is_excluded -> DENY.
#   2. Single command whose first segment is_trusted, redirects safe -> ALLOW.
#   3. Otherwise -> silent; the normal permission flow applies.
#
# Matching is whole-token prefix (basename on token 0), which is
# injection-safe: `git -c x grep` shifts tokens so it matches no trusted
# tuple. ALLOW only ever fires for a single trusted command, and a
# `> file` outside the write roots falls through to a prompt. Missing or
# malformed data files contribute nothing (fail closed).

import json
import os
import re
import shlex
import sys


def _data_dir():
    return os.environ.get("BASH_ALLOW_DIR") or os.path.dirname(os.path.realpath(__file__))


def _load_one(path):
    try:
        with open(path) as f:
            data = json.load(f)
    except (OSError, ValueError):
        return [], []
    def ok(t):
        return isinstance(t, list) and t and all(isinstance(x, str) for x in t)

    excl = [list(t) for t in data.get("excluded", []) if ok(t)]
    trust = [list(t) for t in data.get("trusted", []) if ok(t)]
    return excl, trust


def _dedup(tuples):
    seen, out = set(), []
    for t in tuples:
        key = tuple(t)
        if key in seen:
            continue
        seen.add(key)
        out.append(t)
    return out


def _load():
    d = _data_dir()
    be, bt = _load_one(os.path.join(d, "bash-allow-trusted.json"))
    le, lt = _load_one(os.path.join(d, "bash-allow-trusted.local.json"))
    return _dedup(be + le), _dedup(bt + lt)


EXCLUDED, TRUSTED = _load()

SEPARATORS = frozenset({"|", "||", "&&", ";", ";;", "&", "|&"})
ASSIGN_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
SAFE_REDIRECT_TARGETS = frozenset({"/dev/null", "/dev/stdout", "/dev/stderr"})

WRITE_ROOT_ENV = (("WORKTREES_ROOT", None), ("REPOS_ROOT", None), ("HOME", ".melvin/config"))
FIXED_WRITE_ROOTS = ("/tmp", "/private/tmp")


def write_roots():
    roots = [os.path.realpath(r) for r in FIXED_WRITE_ROOTS]
    for var, sub in WRITE_ROOT_ENV:
        value = os.environ.get(var, "").strip()
        if not value:
            continue
        roots.append(os.path.realpath(os.path.join(value, sub) if sub else value))
    # /tmp and /private/tmp realpath to the same dir on macOS; dedupe so the
    # same root isn't checked twice.
    return list(dict.fromkeys(roots))


def under_write_root(target, cwd, roots):
    if not target:
        return False
    path = os.path.expanduser(target)
    if not os.path.isabs(path):
        path = os.path.join(cwd, path)
    try:
        real = os.path.realpath(path)
    except OSError:
        return False
    for root in roots:
        try:
            if os.path.commonpath([real, root]) == root:
                return True
        except ValueError:
            continue
    return False


def has_unsafe_write_redirect(cmd, cwd, roots):
    i, n = 0, len(cmd)
    in_single = in_double = False
    while i < n:
        ch = cmd[i]
        if in_single:
            if ch == "'":
                in_single = False
            i += 1
            continue
        if in_double:
            if ch == "\\" and i + 1 < n:
                i += 2
                continue
            if ch == '"':
                in_double = False
            i += 1
            continue
        if ch == "\\" and i + 1 < n:
            i += 2
            continue
        if ch == "'":
            in_single = True
            i += 1
            continue
        if ch == '"':
            in_double = True
            i += 1
            continue
        if ch == ">":
            j = i + 1
            if j < n and cmd[j] == ">":
                j += 1
            if j < n and cmd[j] == "&":
                # `>&word` is an fd dup/close ONLY when the whole word is digits
                # (or `-`); otherwise it redirects BOTH stdout and stderr to the
                # FILE word. Capture the full word so `>&1/../etc/x` isn't
                # mistaken for a dup on its leading digit and skipped.
                w = j + 1
                while w < n and cmd[w] not in " \t|&;<>()":
                    w += 1
                word = cmd[j + 1 : w]
                if word == "" or word == "-" or word.isdigit():
                    i = w
                    continue
                j += 1  # filename form: fall through to the write-root check
            while j < n and cmd[j] in " \t":
                j += 1
            k = j
            while k < n and cmd[k] not in " \t|&;<>()":
                k += 1
            target = cmd[j:k]
            # The scan captured surrounding quotes verbatim; strip one matching
            # pair so a quoted absolute path is judged on its real value rather
            # than as a relative-to-cwd string (`> "/etc/hosts"`).
            if len(target) >= 2 and target[0] == target[-1] and target[0] in "'\"":
                target = target[1:-1]
            # Fail closed on a target we cannot statically resolve — a stray
            # quote, escape, or $VAR could expand anywhere outside the write
            # roots, so force a prompt instead of auto-allowing.
            if not target or any(c in target for c in "'\"$`\\"):
                return True
            if target not in SAFE_REDIRECT_TARGETS and not under_write_root(target, cwd, roots):
                return True
            i = k
            continue
        i += 1
    return False


def peel_assignments(tokens):
    i = 0
    while i < len(tokens) and ASSIGN_RE.match(tokens[i]):
        i += 1
    return tokens[i:]


def has_unquoted_newline(cmd):
    # shlex(whitespace_split) collapses a raw newline into ordinary token
    # whitespace, so a multi-line blob looks like one segment. Detect an
    # unquoted newline ourselves so it counts as a command separator.
    in_single = in_double = False
    i, n = 0, len(cmd)
    while i < n:
        ch = cmd[i]
        if ch == "\\" and not in_single and i + 1 < n:
            i += 2
            continue
        if ch == "'" and not in_double:
            in_single = not in_single
        elif ch == '"' and not in_single:
            in_double = not in_double
        elif ch == "\n" and not in_single and not in_double:
            return True
        i += 1
    return False


def split_segments(tokens):
    segments, current = [], []
    for tok in tokens:
        if tok in SEPARATORS:
            if current:
                segments.append(current)
                current = []
        else:
            current.append(tok)
    if current:
        segments.append(current)
    return segments


def prefix_match(prefix, tokens):
    if not tokens:
        return False
    head = [os.path.basename(tokens[0])] + list(tokens[1:])
    return len(head) >= len(prefix) and head[: len(prefix)] == prefix


def is_excluded(tokens):
    return any(prefix_match(p, tokens) for p in EXCLUDED)


def is_trusted(tokens):
    return any(prefix_match(p, tokens) for p in TRUSTED)


def emit(decision, reason):
    json.dump(
        {"hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
            "permissionDecisionReason": reason,
        }},
        sys.stdout,
    )


DENY_REASON = (
    "Excluded commands run unsandboxed and must run ALONE — no pipe (|) or "
    "chain (&&, ||, ;, newline). Redirect to a file, then process it in a "
    "SEPARATE command. Instead of `<cmd> | head -40`, run `<cmd> > /tmp/out.txt`, "
    "then `cat /tmp/out.txt | head -40`."
)


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd:
        return
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    raw_segments = split_segments(tokens)
    segments = [peel_assignments(seg) for seg in raw_segments]
    # Compound only when a separator actually splits two commands (a trailing
    # `&`/`|` leaves one segment) or an unquoted newline joins them. cmd is
    # stripped so a mere trailing newline doesn't count as a separator.
    compound = len(segments) > 1 or has_unquoted_newline(cmd.strip())

    # Deny a compound containing an excluded segment even when it also
    # contains a command substitution — denying is always safe, so this runs
    # before the substitution bail that guards only the ALLOW path.
    if compound:
        if any(is_excluded(seg) for seg in segments):
            emit("deny", DENY_REASON)
        return

    # Never ALLOW a command/process substitution: we cannot reason about what
    # it expands to or spawns, so fall through to the normal permission prompt.
    if "$(" in cmd or "`" in cmd or "<(" in cmd or ">(" in cmd:
        return

    if not segments or not is_trusted(segments[0]):
        return
    # Refuse to auto-allow ANY env-assignment-prefixed command. git/gh honor an
    # open-ended set of env vars that inject commands (GIT_CONFIG_* sets arbitrary
    # config like core.pager/core.sshCommand; GIT_EXTERNAL_DIFF/GIT_PAGER/
    # GIT_SSH_COMMAND/… name programs directly). A denylist of names keeps missing
    # new ones, so treat any leading assignment as unresolvable and fall through
    # to the normal permission prompt.
    if raw_segments[0] and ASSIGN_RE.match(raw_segments[0][0]):
        return
    cwd = (data.get("cwd") or "").strip() or os.getcwd()
    if has_unsafe_write_redirect(cmd, cwd, write_roots()):
        return
    emit("allow", "lone trusted excluded command")


if __name__ == "__main__":
    main()
