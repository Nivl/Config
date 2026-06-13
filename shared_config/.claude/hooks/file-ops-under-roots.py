#!/usr/bin/env python3
# PreToolUse hook: auto-allow rm / rmdir / unlink / cp / mv when every
# positional path is contained under an allowed root: /tmp (and its
# canonical form /private/tmp), $WORKTREES_ROOT, and $REPOS_ROOT.
# Anything else (a path outside the roots, a path we can't resolve, no
# paths at all) falls through to ask. This gives the rm/cp/mv family a
# real path check where settings.json's pattern rules can only match
# the command string. Unlike write-under-roots.py / cd-under-roots.py,
# ~/.melvin/config is deliberately NOT a root here: auto-allowing
# rm/mv there would let the live guard hooks be removed without a
# prompt.
#
# Containment is verified twice for each path: once after lexical
# normalisation (collapses `..` and `.`) and once after realpath
# (resolves symlinks). Both must remain inside the allowed roots, so
# `rm /tmp/..` (lexical escape) and `rm /tmp/symlink-to-etc/x` (symlink
# escape) both go to ask. Relative paths are joined against the hook
# input's `cwd` — a subagent's Bash commands run from the session's
# cwd, not the hook process's getcwd(), so when the input carries no
# cwd a relative path asks rather than guessing from getcwd() (the
# guess usually lands inside a repo under $REPOS_ROOT and would fail
# open).
#
# .git carve-out, mirroring write-under-roots.py: a path that traverses
# a `.git` segment (case-insensitive, checked lexically and after
# resolution) downgrades to ask.
#
# The rm family gets three extra guards, because deletion is the one
# irreversible operation here. A target that IS an allowed root asks
# (`rm -rf /tmp` is never routine). A target that IS a git repo root (a
# directory holding a .git entry — a dir for a repo, a file for a
# linked worktree) asks. And under $WORKTREES_ROOT / $REPOS_ROOT an rm
# target must be INSIDE a repo (some parent up to the dev root holds a
# .git entry) — a cheap recoverability proxy: tracked content is
# restorable, while untracked or ignored files are still lost. The
# roots themselves, org-level dirs full of checkouts, and loose
# non-repo files all ask. /tmp has no inside-a-repo requirement. Stat
# failures during these checks fail toward ask, never allow.
#
# mv SOURCES under the dev roots get the same allowed-root-itself and
# inside-a-repo guards: moving a dir out of a dev root removes it just
# like a delete, and staging through /tmp ('mv <org dir> /tmp/x' then
# 'rm -rf /tmp/x') would otherwise dodge the rm guard with zero
# prompts. Moving a plain repo root stays allowed — .git travels with
# the tree, and rm of the moved repo still asks via the repo-root
# guard — but moving an allowed root itself still asks.
#
# cp/mv specifics: ALL positionals must be inside the roots — sources
# too, so `cp /etc/passwd /tmp/x` asks. GNU `-t` / `--target-directory`
# / `-S` / `--suffix` forms ask as well: they carry the write
# destination inside an option token that the positional scan skips.
#
# Hidden-path forms (command substitution, backticks, process
# substitution) ask too: we can't statically tell what gets touched.
# Wrapper-prefixed forms like `sudo rm` are out of scope here —
# settings.json patterns and the sibling hooks key off the first
# token, and this hook keeps that contract. Same for `xargs rm` and
# `find ... -exec rm`.

import json
import os
import shlex
import stat
import sys

RM_COMMANDS = frozenset({"rm", "rmdir", "unlink"})
COPY_COMMANDS = frozenset({"cp", "mv"})
FILE_OP_COMMANDS = RM_COMMANDS | COPY_COMMANDS
# Pathy invocations are only honored from these dirs. Anything else
# (/tmp/evil/rm, ./rm) is an arbitrary binary that happens to share a
# file-op name — those stay silent and hit the default prompt.
TRUSTED_BIN_DIRS = frozenset({"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/usr/local/bin", "/opt/homebrew/bin"})
FIXED_ROOTS = ("/tmp", "/private/tmp")
ROOT_ENV_VARS = ("WORKTREES_ROOT", "REPOS_ROOT")
# Shell constructs whose expansion we can't see from a static command
# string. Detected on the raw command after confirming the leading
# token, since shlex with punctuation_chars=True splits `$(` and `<(`
# into separate tokens that would otherwise hide these markers. The
# bare "$" subsumes $(...) and ${...} as well as plain $VAR.
HIDDEN_MARKERS = ("$", "`", "<(", ">(")

# Shell control operators that chain a second command or redirect I/O.
# An allow on `rm /tmp/x; curl evil` would also approve the chained
# command, so any of these forces an ask. Detected on shlex tokens so a
# quoted occurrence (e.g. a filename literally containing `;`) doesn't
# trigger. shlex's punctuation_chars mode groups RUNS of `();<>|&` into
# single tokens (`|&`, `>&`, `>|`, ...), so the check below also treats
# any token built entirely from those characters as a control operator
# rather than trusting this enumeration to be complete.
CONTROL_TOKENS = frozenset({
    ";", "&", "&&", "||", "|",
    ">", ">>", "<", "<<", "<<<",
    "(", ")", "{", "}",
})
PUNCTUATION_CHARS = frozenset("();<>|&")


def is_control_token(token: str) -> bool:
    # The bool() guard keeps an empty token (a quoted empty argument)
    # from matching vacuously — it's a path argument, not an operator.
    return token in CONTROL_TOKENS or (
        bool(token) and all(c in PUNCTUATION_CHARS for c in token)
    )


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


def allowed_roots():
    # Two root sets so each containment check compares like-with-like:
    # lexical paths against normalized roots, resolved paths against
    # realpathed roots (on macOS /tmp itself is a symlink). The env-var
    # roots are also returned separately (resolved) — the inside-a-repo
    # guard (rm targets and mv sources) applies only under those.
    lexical = [os.path.normpath(r) for r in FIXED_ROOTS]
    env_resolved = []
    for var in ROOT_ENV_VARS:
        value = os.environ.get(var, "").strip()
        if value:
            norm = os.path.normpath(value)
            lexical.append(norm)
            env_resolved.append(os.path.realpath(norm))
    resolved = sorted({os.path.realpath(r) for r in lexical})
    return lexical, resolved, env_resolved


def repo_root_state(path):
    # True: path is a directory holding a .git entry (repo root, or a
    # linked worktree whose .git is a file). False: definitely not.
    # OSError instance: could not inspect. os.path.isdir/exists would
    # swallow EACCES/ELOOP as False, which here would shift the decision
    # toward allow — return the error so the caller asks and can name
    # the cause.
    try:
        st = os.stat(path)
    except FileNotFoundError:
        return False
    except OSError as e:
        return e
    if not stat.S_ISDIR(st.st_mode):
        return False
    try:
        os.lstat(os.path.join(path, ".git"))
    except FileNotFoundError:
        return False
    except OSError as e:
        return e
    return True


def inside_repo(real, root):
    # Walk parent dirs from the target's directory up to (and including)
    # the containing dev root, looking for a .git entry — found means
    # the target lives in a git working tree, the cheap recoverability
    # proxy described in the header. Stat errors count as not-found, so
    # the walk fails toward ask.
    cur = os.path.dirname(real)
    while len(cur) >= len(root):
        try:
            os.lstat(os.path.join(cur, ".git"))
            return True
        except OSError:
            pass
        if cur == root:
            return False
        cur = os.path.dirname(cur)
    return False


def collect_targets(args):
    # Skip option tokens and return the positional targets. `--` ends
    # option parsing, so any later token is a target even if it starts
    # with `-` (so `rm -- -weird-file` works). A lone `-` is a positional
    # operand by getopt convention (a file literally named "-"), not an
    # option — skipping it would leave an unvetted target.
    targets = []
    in_options = True
    for a in args:
        if in_options and a == "--":
            in_options = False
            continue
        if in_options and a.startswith("-") and a != "-":
            continue
        targets.append(a)
    return targets


def has_destination_option(args):
    # GNU cp/mv can carry the destination (or a backup suffix that
    # becomes part of a write path) inside an option: `-t DIR`,
    # `--target-directory=DIR`, `-S SUF`, `--suffix=SUF`, including
    # clustered (`-rt DIR`) and attached (`-S.bak`) short forms. The
    # positional scan can't see those paths, so their presence forces
    # an ask. GNU getopt_long also accepts unambiguous abbreviations
    # (`--target=DIR`), so we flag any long option whose name is a
    # prefix of the full spelling rather than prefix-matching the other
    # way. Tokens after `--` are positionals and don't count.
    for a in args:
        if a == "--":
            return False
        if not a.startswith("-"):
            continue
        if a.startswith("--"):
            opt = a.split("=", 1)[0]
            if len(opt) > 2 and (
                "--target-directory".startswith(opt) or "--suffix".startswith(opt)
            ):
                return True
        elif any(c in "tS" for c in a[1:]):
            return True
    return False


def has_hidden_expansion(s: str) -> bool:
    return any(m in s for m in HIDDEN_MARKERS)


GLOB_CHARS = frozenset("*?[")


def has_unverifiable_expansion(token: str) -> bool:
    # Bash expands brace groups and globs AFTER this static check, so a
    # token still carrying them describes paths we can't verify: the
    # `..` in `/tmp/{a,../etc/passwd}` hides inside one path component
    # where normpath can't collapse it, and a glob like `.g*` can match
    # a .git entry. A brace group only expands when its body has a `,`
    # or a `..` sequence — `{a}` stays literal. shlex strips quotes
    # before we see the token, so a quoted glob-char filename over-asks,
    # which is the safe direction.
    if any(c in GLOB_CHARS for c in token):
        return True
    i = token.find("{")
    while i != -1:
        j = token.find("}", i + 1)
        if j == -1:
            return False
        body = token[i + 1 : j]
        if "," in body or ".." in body:
            return True
        i = token.find("{", j + 1)
    return False


def under_any_root(path: str, roots) -> bool:
    for root in roots:
        try:
            if os.path.commonpath([path, root]) == root:
                return True
        except ValueError:
            # Mixed absolute/relative, or paths on different drives.
            continue
    return False


def has_git_segment(path: str) -> bool:
    # Lowercase before comparing — macOS APFS is case-insensitive by
    # default, so `.GIT/HEAD` resolves to the same dir as `.git/HEAD`.
    return ".git" in [p.lower() for p in path.split(os.sep)]


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    if not isinstance(data, dict):
        # Valid JSON that isn't an object (null, [], a string) would
        # crash the .get below — bail silently like any unreadable
        # payload, so the default permission flow takes over.
        return

    tool_input = data.get("tool_input")
    cmd = (tool_input.get("command") if isinstance(tool_input, dict) else "") or ""
    if not cmd:
        return

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if not tokens:
        return
    head = tokens[0]
    name = os.path.basename(head)
    if name not in FILE_OP_COMMANDS:
        return
    if os.sep in head and os.path.dirname(os.path.normpath(head)) not in TRUSTED_BIN_DIRS:
        return

    if has_hidden_expansion(cmd):
        emit(
            "ask",
            f"{name} command uses shell expansion ($(...), ${{VAR}}, backticks, "
            "<(...) etc.) that can't be checked statically.",
        )
        return

    # A control operator anywhere in the rest of the stream means the agent
    # is chaining another command, piping, or redirecting. An `allow` here
    # would approve the chained / redirected operation too, so refuse.
    bad = next((t for t in tokens[1:] if is_control_token(t)), None)
    if bad is not None:
        emit(
            "ask",
            f"{name} command contains a shell control operator ({bad!r}); "
            "split into separate Bash calls to keep its scope inspectable.",
        )
        return
    # Embedded newlines / carriage returns are also chain operators that
    # shlex collapses into whitespace, so check the raw command too.
    if "\n" in cmd or "\r" in cmd:
        emit(
            "ask",
            f"{name} command spans multiple lines; split into separate Bash calls.",
        )
        return

    if name in COPY_COMMANDS and has_destination_option(tokens[1:]):
        emit(
            "ask",
            f"{name} uses -t/--target-directory or -S/--suffix, which hide a "
            "write path inside an option; spell the destination as a "
            "positional argument instead.",
        )
        return

    targets = collect_targets(tokens[1:])
    if not targets:
        emit("ask", f"{name} invocation has no static target — asking for explicit approval.")
        return

    cwd = str(data.get("cwd") or "").strip()
    lexical_roots, real_roots, env_real_roots = allowed_roots()
    # For mv, every positional except the last is a source being removed
    # from its directory — destructive like rm from the dev roots'
    # perspective, so sources share rm's recoverability guard below.
    mv_sources = frozenset(targets[:-1]) if name == "mv" and len(targets) > 1 else frozenset()

    for target in targets:
        if not target:
            # A quoted empty argument joins toward cwd under path math,
            # which could read as contained — call it out instead.
            emit("ask", f"{name} command has an empty path argument.")
            return
        if has_unverifiable_expansion(target):
            emit(
                "ask",
                f"{name} path {target!r} carries brace-expansion or glob "
                "characters whose runtime expansion can't be checked "
                "statically.",
            )
            return
        # Tilde-expand so `rm ~/foo` resolves to the actual home path rather
        # than being joined naively under cwd.
        expanded = os.path.expanduser(target)
        # expanduser only resolves `~` and `~user`. Bash also expands
        # `~+`/`~-` (PWD/OLDPWD) and the dirstack forms `~N`/`~+N`/`~-N`,
        # which expanduser leaves starting with `~` — joining those under
        # cwd would mask a target that bash sends outside the roots
        # (e.g. `rm ~-/x` deletes $OLDPWD/x). A leftover leading `~`
        # (also an unknown ~user) is unresolvable here, so ask.
        if expanded.startswith("~"):
            emit(
                "ask",
                f"{name} path {target!r} uses a tilde form that can't be "
                "resolved statically (~+/~-/~N dirstack or unknown user).",
            )
            return
        if os.path.isabs(expanded):
            absolute = expanded
        elif cwd:
            absolute = os.path.join(cwd, expanded)
        else:
            emit(
                "ask",
                f"{name} path {target!r} is relative but the hook input "
                "has no cwd to resolve it against.",
            )
            return
        lexical = os.path.normpath(absolute)
        # Containment resolution depends on the command. rm operates on
        # the link itself, so it gets parent-only resolution: `rm
        # /tmp/symlink-to-file` (deleting the symlink) stays an allow
        # while `rm /tmp/symlink-dir/x` (descending into the symlink,
        # which lands outside) becomes an ask. cp and mv FOLLOW a
        # final-component symlink for reads and writes, so containment
        # uses the full realpath for them. The recoverability guards
        # below use `location` (parent-only) for every command — mv
        # moves the link itself, so what matters there is where the
        # operand LIVES, not what it points at.
        parent_real = os.path.realpath(os.path.dirname(absolute) or "/")
        location = os.path.join(parent_real, os.path.basename(absolute))
        real = os.path.realpath(absolute) if name in COPY_COMMANDS else location

        if not under_any_root(lexical, lexical_roots):
            emit(
                "ask",
                f"{name} path {target!r} resolves outside the allowed roots "
                f"(lexical path {lexical!r}).",
            )
            return
        if not under_any_root(real, real_roots):
            emit(
                "ask",
                f"{name} path {target!r} resolves outside the allowed roots "
                f"via symlink (realpath {real!r}).",
            )
            return
        if has_git_segment(lexical) or has_git_segment(real):
            emit(
                "ask",
                f"{name} path {target!r} traverses a .git directory; "
                "refusing to auto-allow.",
            )
            return
        if (name in RM_COMMANDS or target in mv_sources) and (
            lexical in lexical_roots or real in real_roots
        ):
            # Removing or relocating a whole allowed root is never routine.
            # Without this, `rm -rf /private/tmp` and `mv /private/tmp x`
            # would auto-allow (the dev roots already ask via the
            # inside-a-repo guard, /tmp only by the accident of its
            # /private/tmp symlinking).
            emit(
                "ask",
                f"{name} path {target!r} is an allowed root itself; "
                "asking for explicit approval.",
            )
            return

        if name in RM_COMMANDS or target in mv_sources:
            # A symlink operand is just a link: rm unlinks it and mv
            # relocates it, so it is never a repo root for these checks
            # (stat would follow it and misread the link as the repo),
            # and the place that must be recoverable is where the LINK
            # lives — a link in an org dir pointing into a repo must not
            # borrow the repo's recoverability.
            state = False if os.path.islink(location) else repo_root_state(location)
            if isinstance(state, OSError):
                emit(
                    "ask",
                    f"could not inspect {name} path {target!r} "
                    f"({state.strerror or state}); asking for explicit "
                    "approval.",
                )
                return
            if state:
                if name in RM_COMMANDS:
                    emit(
                        "ask",
                        f"rm target {target!r} is a git repository root; "
                        "deleting it is irreversible, asking for explicit "
                        "approval.",
                    )
                    return
                # An mv source that IS a repo root stays allowed: .git
                # travels with the tree, and rm of the moved repo still
                # asks via this same guard.
            else:
                for env_root in env_real_roots:
                    if under_any_root(location, [env_root]) and not inside_repo(location, env_root):
                        emit(
                            "ask",
                            f"{name} path {target!r} is under {env_root} but "
                            "not inside a git repository, so the operation "
                            "is not git-recoverable; asking for explicit "
                            "approval.",
                        )
                        return

    emit("allow", f"all {name} paths are inside the allowed roots")


if __name__ == "__main__":
    main()
