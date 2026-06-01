#!/usr/bin/env python3
# PreToolUse hook: auto-allow rm / rmdir / unlink when every positional
# target is contained under /tmp or /private/tmp. Anything else (target
# outside the allowed roots, target unresolvable, no targets at all)
# falls through to ask. Replaces the pattern-matched `Bash(rm *)` and
# `Bash(/bin/rm *)` ask entries in settings.json with a real path check.
#
# Containment is verified twice for each target: once after lexical
# normalisation (collapses `..` and `.`) and once after realpath
# (resolves symlinks). Both must remain inside the allowed roots, so
# `rm /tmp/..` (lexical escape) and `rm /tmp/symlink-to-etc/x` (symlink
# escape) both go to ask. Relative paths are joined against the hook
# input's `cwd` first — the Write tool's subagents send relative paths
# resolved against the session cwd, not the hook process's getcwd().
#
# Hidden-path forms (command substitution, backticks, process
# substitution) ask too: we can't statically tell what gets deleted.
# Wrapper-prefixed forms like `sudo rm` are out of scope here — they
# never matched `Bash(rm *)` either, since settings.json patterns key
# off the first token. Same for `xargs rm` and `find ... -exec rm`.

import json
import os
import shlex
import sys

RM_COMMANDS = frozenset({"rm", "rmdir", "unlink"})
FIXED_ROOTS = ("/tmp", "/private/tmp")
# Shell constructs whose expansion we can't see from a static command
# string. Detected on the raw command after confirming we're looking at
# rm, since shlex with punctuation_chars=True splits `$(` and `<(` into
# separate tokens that would otherwise hide these markers.
HIDDEN_MARKERS = ("$(", "`", "<(", ">(", "${", "$")

# Shell control operators that chain a second command onto rm or redirect
# I/O. An allow on `rm /tmp/x; curl evil` would also approve the chained
# command, so any of these forces an ask. Detected on shlex tokens so a
# quoted occurrence (e.g. a filename literally containing `;`) doesn't
# trigger.
CONTROL_TOKENS = frozenset({
    ";", "&", "&&", "||", "|",
    ">", ">>", "<", "<<", "<<<", "2>", "2>>", "&>", "&>>",
    "(", ")", "{", "}",
})


def emit(decision: str, reason: str) -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": decision,
                "permissionDecisionReason": reason,
            }
        },
        sys.stdout,
    )


def collect_targets(args):
    # Skip option tokens and return the positional targets. `--` ends
    # option parsing, so any later token is a target even if it starts
    # with `-` (so `rm -- -weird-file` works).
    targets = []
    in_options = True
    for a in args:
        if in_options and a == "--":
            in_options = False
            continue
        if in_options and a.startswith("-"):
            continue
        targets.append(a)
    return targets


def has_hidden_expansion(s: str) -> bool:
    return any(m in s for m in HIDDEN_MARKERS)


def under_any_root(path: str, roots) -> bool:
    for root in roots:
        try:
            if os.path.commonpath([path, root]) == root:
                return True
        except ValueError:
            # Mixed absolute/relative, or paths on different drives.
            continue
    return False


def main() -> None:
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

    if not tokens or os.path.basename(tokens[0]) not in RM_COMMANDS:
        return

    if has_hidden_expansion(cmd):
        emit(
            "ask",
            "rm command uses shell expansion ($(...), ${VAR}, backticks, "
            "<(...) etc.) that can't be checked statically.",
        )
        return

    # A control operator anywhere in the rest of the stream means the agent
    # is chaining another command, piping, or redirecting. An `allow` here
    # would approve the chained / redirected operation too, so refuse.
    bad = next((t for t in tokens[1:] if t in CONTROL_TOKENS), None)
    if bad is not None:
        emit(
            "ask",
            f"rm command contains a shell control operator ({bad!r}); "
            "split into separate Bash calls to keep the rm scope inspectable.",
        )
        return
    # Embedded newlines / carriage returns are also chain operators that
    # shlex collapses into whitespace, so check the raw command too.
    if "\n" in cmd or "\r" in cmd:
        emit(
            "ask",
            "rm command spans multiple lines; split into separate Bash calls.",
        )
        return

    targets = collect_targets(tokens[1:])
    if not targets:
        emit("ask", "rm invocation has no static target — asking for explicit approval.")
        return

    cwd = (data.get("cwd") or "").strip() or os.getcwd()
    # Two root sets so each containment check compares like-with-like.
    # macOS makes /tmp a symlink to /private/tmp, so the resolved set is
    # narrower than the lexical one. realpath the parent dir only — never
    # the final component — so `rm /tmp/symlink-to-file` (deleting the
    # symlink itself) stays an allow while `rm /tmp/symlink-dir/x`
    # (descending into the symlink, which lands outside) becomes an ask.
    lexical_roots = [os.path.normpath(r) for r in FIXED_ROOTS]
    real_roots = sorted({os.path.realpath(r) for r in FIXED_ROOTS})

    for target in targets:
        # Tilde-expand so `rm ~/foo` resolves to the actual home path rather
        # than being joined naively under cwd.
        expanded = os.path.expanduser(target)
        absolute = expanded if os.path.isabs(expanded) else os.path.join(cwd, expanded)
        lexical = os.path.normpath(absolute)
        parent_real = os.path.realpath(os.path.dirname(absolute) or "/")
        real = os.path.join(parent_real, os.path.basename(absolute))

        if not under_any_root(lexical, lexical_roots):
            emit(
                "ask",
                f"rm target {target!r} resolves outside /tmp (lexical path {lexical!r}).",
            )
            return
        if not under_any_root(real, real_roots):
            emit(
                "ask",
                f"rm target {target!r} resolves outside /tmp via symlink (realpath {real!r}).",
            )
            return

    emit("allow", "all rm targets are inside /tmp")


if __name__ == "__main__":
    main()
