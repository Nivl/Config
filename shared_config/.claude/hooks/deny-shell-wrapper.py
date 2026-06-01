#!/usr/bin/env python3
# PreToolUse hook: deny Bash invocations whose first command is "inline code"
# rather than a real, discrete command.
#
# The Bash tool already runs in a shell, so anything that wraps logic into one
# opaque blob defeats the rest of the safety net: the permission allowlist and
# the sibling hooks (cd, git, gh, write) all key off the FIRST token of the
# command. If the agent buries the real work inside a function body, a loop,
# a heredoc, an interpreter -c/-e/--eval string, or a multi-line ANSI-C quoted
# string, those checks see only `bash` / `python` / `for` and let the rest
# through.
#
# Rules covered (each denies with its own reason):
#   - Leading shell `-c` / `-lc` / `-ec` style.
#   - Leading interpreter code-string flag: python `-c`, perl `-e`/`-E`,
#     ruby `-e`, node `-e`/`-p`/`--eval`/`--print`, deno `eval`.
#   - Function declarations: `name() { ... }` or `function name { ... }`.
#   - for/while/until/select loops with a do...done body.
#   - `eval`, `source`, or `.` at command-position.
#   - Heredocs / here-strings fed into an interpreter, including via pipe.
#   - Multi-line ANSI-C `$'...\n...'` strings.
#
# Only the LEADING command is inspected. Embedded uses like `xargs python -c`
# or `timeout bash -c` are left alone — they have legitimate composition uses
# and the agent is not authoring code inline there. awk/gawk are excluded from
# the interpreter list because their first positional arg is always a program
# string by design — blocking would break normal usage.

import json
import os
import re
import shlex
import sys

SHELLS = {"bash", "sh", "zsh", "dash", "ksh", "mksh", "ash", "pwsh", "powershell"}
# `^python$`, `^python3$`, `^python3.11$` match; trailing-dot variants
# (`python.`, `python3.`) and arbitrary garbage don't. pypy follows the same
# shape (pypy, pypy3, pypy3.10).
PYTHON_RE = re.compile(r"^python(?:\d+(?:\.\d+)?)?$")
PYPY_RE = re.compile(r"^pypy(?:\d+(?:\.\d+)?)?$")
IDENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
ASSIGNMENT_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")

# Wrappers that pass their tail through to a child command. The leading-token
# rules (R0 shell -c, R1 interpreter -c) need to look past these — otherwise
# `env python3 -c '...'` and `sudo bash -c '...'` defeat the entire check.
# `xargs` and `timeout` are deliberately NOT in this set: they're composition
# wrappers (legitimate orchestration), not invocation wrappers.
WRAPPER_PREFIXES = frozenset({
    "env", "sudo", "doas", "command", "exec",
    "nice", "ionice", "nohup", "time", "taskset", "setsid",
    "stdbuf", "chronic", "busybox",
})

# Flags that take a separate-token argument, per wrapper. A flat shared set
# over-strips (e.g. `sudo -S` and `time -p` are no-arg flags, not arg-takers).
# Each entry holds both short forms (`-u`) and long forms (`--user`); the
# `--flag=value` form is already a single shlex token and doesn't need to be
# listed here.
WRAPPER_ARG_FLAGS = {
    "env":     frozenset({"-u", "-S", "-C", "--unset", "--split-string", "--chdir"}),
    "sudo":    frozenset({"-u", "-g", "-p", "-C", "-a", "-r", "-t", "-h", "-D",
                          "--user", "--group", "--prompt", "--chdir", "--role",
                          "--type", "--host", "--close-from", "--auth-type"}),
    "doas":    frozenset({"-u", "-C", "--user"}),
    "command": frozenset(),
    "exec":    frozenset({"-a"}),
    "nice":    frozenset({"-n", "--adjustment"}),
    "ionice":  frozenset({"-c", "-n", "-p", "--class", "--classdata", "--pid"}),
    "nohup":   frozenset(),
    "time":    frozenset(),
    "taskset": frozenset({"-c", "-p"}),
    "setsid":  frozenset(),
    "stdbuf":  frozenset({"-i", "-o", "-e", "--input", "--output", "--error"}),
    "chronic": frozenset(),
    "busybox": frozenset(),
}

# Shell options that take a separate-token argument (`bash -o pipefail -c …`,
# `zsh -O extglob -c …`). Used by uses_dash_c to walk past option-arg pairs
# instead of bailing on the first non-`-` token.
SHELL_ARG_FLAGS = frozenset({"-o", "+o", "-O", "+O"})

# basename -> (short flag characters, long flag names) that trigger code-string
# mode for that interpreter. Python is handled via PYTHON_RE because its name
# carries an optional version suffix (python3, python3.11).
# For each interpreter: (code-string short chars, code-string long flags,
# arg-swallowing short chars). When walking a short cluster we match on any
# code-string char, but a char in `arg_swallowing` ends the cluster early
# (the rest is that flag's argument). This is how `ruby -ractive_support`
# stops at `r` instead of falsely matching the `e` later in `active_support`.
INTERPRETER_FLAGS = {
    "perl":   ("eE", (),                          "iI"),
    "ruby":   ("e",  (),                          "rIKCFTW"),
    "node":   ("ep", ("--eval", "--print"),       "r"),
    "nodejs": ("ep", ("--eval", "--print"),       "r"),
    "bun":    ("e",  ("--eval", "--print"),       "r"),
}

LOOP_KEYWORDS = {"for", "while", "until", "select"}
# Tokens that introduce a new simple command on their right side — i.e.
# anything where a keyword sitting just after it is at "command-position"
# rather than mid-argument. Combines (a) plain separators (`;`, `&&`, `|`),
# (b) compound-statement keywords (`then`, `do`, `else`, `elif`), (c) group
# openers/closers (`(`, `)`, `{`, `}`), (d) case-arm break (`;;`), and (e)
# negation (`!`). Used by has_loop, has_sourcing_call, and the per-simple-
# command splitter for R0/R1.
CMD_SEPARATORS = frozenset({
    ";", "|", "&", "&&", "||",
    "(", "{", "!",
    "then", "else", "elif", "do",
    ")", "}", ";;",
})

# Builtins that hide the real command from the allowlist and sibling hooks.
# `eval CMD` is `bash -c CMD` in builtin form. `source FILE` / `. FILE` run a
# file whose contents the hooks can't see.
SOURCING_BUILTINS = frozenset({"eval", "source", "."})

# Interpreters that turn a heredoc body into executable code. Used by both
# the token-based heredoc check (which walks backward from `<<` to find the
# command-position leading token) and the raw-text fallback below.
HEREDOC_INTERPRETERS = frozenset({
    "bash", "sh", "zsh", "dash", "ksh", "mksh", "ash",
    "perl", "ruby", "node", "nodejs", "deno", "bun",
    "pwsh", "powershell",
})

# Loose fallback for the rare case where shlex tokenisation fails (e.g.
# unterminated quote). False positives are acceptable here — unparseable
# input is already an outlier, and the token-based check is the primary one.
HEREDOC_INTERP_FALLBACK_RE = re.compile(
    r"(?:^|[\s;&|`(])"
    r"(?:[\w./-]*/)?"
    r"(?:bash|sh|zsh|dash|ksh|mksh|ash|python[0-9.]*|pypy[0-9.]*|"
    r"perl|ruby|node|nodejs|deno|bun|pwsh|powershell)"
    r"\b[^\n]*<<"
)

# ANSI-C quoted string: $'...' with normal char or escaped char in the body.
ANSI_C_RE = re.compile(r"\$'(?P<body>(?:[^'\\]|\\.)*)'")

REASON_BASH_C = (
    "Don't wrap commands in `bash -c '...'` (or `sh -c` / `zsh -c`). The Bash tool "
    "already runs in a shell, so it is redundant — and it hides the real command "
    "inside an opaque string, so the permission allowlist and the cd / git / gh "
    "hooks (which all key off the first token) never see it. Run the command "
    "directly. To work in another directory, run `cd <path>` as its own Bash call "
    "first — the tool's working directory persists across calls."
)

REASON_INTERP_C = (
    "Don't pass code strings to interpreters via flags like `python -c '...'`, "
    "`perl -e '...'`, `ruby -e '...'`, `node -e '...'` / `node --eval ...`, or "
    "`deno eval ...`. The permission allowlist and the sibling hooks (cd, git, "
    "gh, write) all key off the FIRST token of the command — anything stuffed "
    "inside the code-string argument is invisible to them. Run discrete "
    "commands directly. The Bash tool's working directory persists across "
    "calls, so each call can share state with the previous one."
)

REASON_FUNC = (
    "Don't define shell functions inline in a Bash command (e.g. `name() "
    "{ ... }` or `function name { ... }`). The Bash tool should run discrete "
    "commands the permission allowlist and sibling hooks can inspect. If you "
    "need to repeat logic, run the command N times directly. The Bash tool's "
    "working directory persists across calls, so each call can share state "
    "with the previous one."
)

REASON_LOOP = (
    "Don't write `for` / `while` / `until` / `select` loops inline in a Bash "
    "command. A loop body bundles many invocations into one opaque blob the "
    "permission allowlist and sibling hooks can't inspect. If you need to "
    "repeat an operation, run it N times directly. The Bash tool's working "
    "directory persists across calls, so each call can share state with the "
    "previous one."
)

REASON_HEREDOC = (
    "Don't pipe a heredoc (`<<EOF`) into an interpreter (bash, python, perl, "
    "ruby, node, deno). The script body is invisible to the permission "
    "allowlist and sibling hooks — same problem as `bash -c '...'`. Run "
    "discrete commands directly. The Bash tool's working directory persists "
    "across calls, so each call can share state with the previous one."
)

REASON_EVAL_SOURCE = (
    "Don't use `eval`, `source`, or `.` (POSIX source) to run code. `eval` "
    "runs an arbitrary command string with the same opacity as `bash -c "
    "'...'`. `source` / `.` run a script file whose contents are invisible "
    "to the permission allowlist and sibling hooks. Run discrete commands "
    "directly. The Bash tool's working directory persists across calls, so "
    "each call can share state with the previous one."
)

REASON_ANSI_C = (
    "Don't smuggle multi-line scripts through `$'...\\n...'` ANSI-C-quoted "
    "strings. The newlines turn one argument into multiple statements the "
    "permission allowlist and sibling hooks can't inspect. Run discrete "
    "commands directly. The Bash tool's working directory persists across "
    "calls, so each call can share state with the previous one."
)


def iter_simple_commands(tokens):
    # Split the token stream into simple-command slices on CMD_SEPARATORS.
    # Used by the leading-token rules (R0/R1/R6) so a command position after a
    # `;`, `&&`, `then`, `(`, `!`, etc. gets checked the same way the first
    # token is. Empty slices (consecutive separators) are skipped.
    if not tokens:
        return
    n = len(tokens)
    start = 0
    for i in range(n):
        if tokens[i] in CMD_SEPARATORS:
            if start < i:
                yield tokens[start:i]
            start = i + 1
    if start < n:
        yield tokens[start:n]


def strip_wrappers(tokens):
    # Peel transparent VAR=val assignments and wrapper commands (env, sudo,
    # nice, ...) plus their flags. After peeling, tokens[0] is the command
    # the agent actually wanted to run — that's what the leading-token rules
    # should see. xargs / timeout are not peeled (real composition wrappers).
    i = 0
    n = len(tokens)
    peeled = False
    while i < n:
        t = tokens[i]
        if ASSIGNMENT_RE.match(t):
            i += 1
            peeled = True
            continue
        base = os.path.basename(t)
        if base in WRAPPER_PREFIXES:
            arg_flags = WRAPPER_ARG_FLAGS.get(base, frozenset())
            i += 1
            peeled = True
            while i < n:
                ti = tokens[i]
                if ti == "--":
                    i += 1
                    break
                if ASSIGNMENT_RE.match(ti):
                    i += 1
                    continue
                if ti.startswith("-"):
                    # Strip a `--flag=value` token via the long-flag check
                    # (entire token is one shlex token, no second-token to
                    # consume).
                    bare = ti.split("=", 1)[0]
                    if (ti in arg_flags or bare in arg_flags) and "=" not in ti and i + 1 < n:
                        i += 2
                    else:
                        i += 1
                    continue
                break
            continue
        break
    return tokens[i:] if peeled else tokens


def emit_deny(reason: str) -> None:
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


def uses_dash_c(args) -> bool:
    # A leading `-c` — or a short-flag cluster containing c, e.g. `-lc`, `-ec` —
    # puts the shell in command-string mode: the next argument is a command to
    # run. Walk past option-arg pairs (`-o pipefail`, `-O extglob`) so they
    # don't terminate the scan: an agent that writes `bash -o pipefail -c
    # CMD` would otherwise bypass R0 because `pipefail` looks like the script
    # name. The first non-flag token AFTER all options is the script name, so
    # only bail there.
    skip_next = False
    for a in args:
        if skip_next:
            skip_next = False
            continue
        if a == "--" or not a.startswith("-"):
            return False
        if a.startswith("--"):
            continue
        if a in SHELL_ARG_FLAGS:
            skip_next = True
            continue
        if "c" in a[1:]:
            return True
    return False


def uses_short_or_long(args, short_chars: str, long_flags=(), arg_swallowing: str = "") -> bool:
    # Walk option args. Trigger on any short-flag char in `short_chars` or
    # any long flag in `long_flags`. When walking a short cluster we go
    # left-to-right; a char in `arg_swallowing` ends the cluster early
    # (the rest of THIS token is the flag's packed arg). If that arg-
    # swallowing char is also the last char of the cluster, the flag's arg
    # is in the NEXT positional token — skip that next token instead of
    # bailing on it, so `node -r mod -e 'x'` and `python -W default -c 'x'`
    # don't escape the check.
    skip_next_positional = False
    for a in args:
        if skip_next_positional:
            skip_next_positional = False
            if not a.startswith("-"):
                continue
        if a == "--" or not a.startswith("-"):
            return False
        if a.startswith("--"):
            name = a.split("=", 1)[0]
            if name in long_flags:
                return True
            continue
        cluster = a[1:]
        for ch in cluster:
            if ch in short_chars:
                return True
            if ch in arg_swallowing:
                if cluster[-1] == ch:
                    skip_next_positional = True
                break
    return False


def deno_uses_eval(args) -> bool:
    # `deno` is subcommand-based; `eval` is the code-string subcommand. Skip
    # any leading options and check the first positional token.
    for a in args:
        if a == "--":
            continue
        if a.startswith("-"):
            continue
        return a == "eval"
    return False


def interpreter_code_string(tokens) -> bool:
    # Match on any code-string flag char inside a cluster, but stop walking
    # once an arg-swallowing flag is hit (its packed arg is not more flags).
    # This catches packed forms like `python -cprint(1)` and `perl -eX` while
    # leaving `ruby -ractive_support` and `perl -i.bak` alone.
    if len(tokens) < 2:
        return False
    base = os.path.basename(tokens[0])
    args = tokens[1:]
    if PYTHON_RE.match(base) or PYPY_RE.match(base):
        return uses_short_or_long(args, "c", arg_swallowing="mWX")
    if base in INTERPRETER_FLAGS:
        short, longf, swallow = INTERPRETER_FLAGS[base]
        return uses_short_or_long(args, short, longf, arg_swallowing=swallow)
    if base == "deno":
        return deno_uses_eval(args)
    return False


def has_function_def(tokens) -> bool:
    # With punctuation_chars=True, shlex returns `()` as a single token and
    # `{` as its own token. So a POSIX `foo() { ... }` definition appears as
    # [IDENT, '()', '{']; the bash `function foo { ... }` form as ['function',
    # IDENT, '{'] (optionally with an extra '()' between IDENT and '{').
    n = len(tokens)
    for i, t in enumerate(tokens):
        if t == "function" and i + 2 < n and IDENT_RE.match(tokens[i + 1]):
            nxt = tokens[i + 2]
            if nxt == "{":
                return True
            if nxt == "()" and i + 3 < n and tokens[i + 3] == "{":
                return True
        elif i + 2 < n and IDENT_RE.match(t):
            if tokens[i + 1] == "()" and tokens[i + 2] == "{":
                return True
    return False


def strip_heredoc_bodies_from_tokens(tokens):
    # After shlex tokenises a heredoc, the body shows up as ordinary tokens
    # in the same flat stream as the rest of the command. has_loop and
    # has_function_def shouldn't treat keywords inside a `cat <<EOF` body as
    # part of the running invocation. For each `<<` / `<<-` opener, scan
    # forward for the matching delimiter token and drop everything between
    # them. If no closing delimiter is found (truncated/malformed input),
    # restore the original tokens unchanged — silently swallowing the tail
    # would hide eval/source/etc. that appears after the not-actually-closed
    # heredoc. Imperfect when other tokens share the opener line, but the
    # common case (a heredoc-only body) is handled.
    out = []
    i = 0
    n = len(tokens)
    while i < n:
        t = tokens[i]
        if t in ("<<", "<<-") and i + 1 < n:
            delim = tokens[i + 1].strip("'\"")
            j = i + 2
            while j < n and tokens[j] != delim:
                j += 1
            if j < n:
                out.append(t)
                out.append(tokens[i + 1])
                out.append(tokens[j])
                i = j + 1
            else:
                out.append(t)
                i += 1
        else:
            out.append(t)
            i += 1
    return out


def has_loop(tokens) -> bool:
    # A real loop keyword sits at command-position: start of the token stream,
    # or immediately after a CMD_SEPARATORS token, or immediately after a
    # transparent prefix (invocation wrapper like `time` / `nice`, or a
    # variable assignment like `FOO=bar` — both are environment modifiers,
    # not command-terminators). Otherwise the word is a plain argument
    # (e.g. `echo for; echo do; echo done`). The body must contain `do` and
    # then `done` after the keyword.
    for i, t in enumerate(tokens):
        if t not in LOOP_KEYWORDS:
            continue
        if i > 0:
            prev = tokens[i - 1]
            if (
                prev not in CMD_SEPARATORS
                and os.path.basename(prev) not in WRAPPER_PREFIXES
                and not ASSIGNMENT_RE.match(prev)
            ):
                continue
        try:
            do_idx = tokens.index("do", i + 1)
            tokens.index("done", do_idx + 1)
        except ValueError:
            continue
        return True
    return False


def _is_heredoc_interpreter(token: str) -> bool:
    base = os.path.basename(token)
    if base in HEREDOC_INTERPRETERS:
        return True
    if PYTHON_RE.match(base):
        return True
    return False


def has_shell_dash_c(tokens) -> bool:
    # R0: walk every simple command, peel wrappers, ask if the leading token
    # is a shell with a -c flag. Mid-stream and grouped forms (`; bash -c`,
    # `(bash -c)`, `! bash -c`, `if true; then bash -c …; fi`) all match.
    for simple in iter_simple_commands(tokens):
        peeled = strip_wrappers(simple)
        if peeled and os.path.basename(peeled[0]) in SHELLS and uses_dash_c(peeled[1:]):
            return True
    return False


def has_interpreter_dash_c(tokens) -> bool:
    # R1: same per-simple-command walk for python/perl/ruby/node/deno/bun.
    for simple in iter_simple_commands(tokens):
        peeled = strip_wrappers(simple)
        if peeled and interpreter_code_string(peeled):
            return True
    return False


def has_sourcing_call(tokens) -> bool:
    # R6: eval/source/. at command-position (start of any simple command,
    # after wrappers are peeled). Plain occurrences as arguments (e.g.
    # `find . -type d`, `printf eval`) are skipped.
    for simple in iter_simple_commands(tokens):
        peeled = strip_wrappers(simple)
        if peeled and peeled[0] in SOURCING_BUILTINS:
            return True
    return False


_HEREDOC_OPENERS = ("<<", "<<-", "<<<")


def has_interpreter_heredoc_tokens(tokens) -> bool:
    # For each heredoc / here-string opener (`<<`, `<<-`, `<<<`), walk
    # backwards to find the simple command that opened it, peel wrappers,
    # and check whether the leading token is an interpreter — catches
    # `python3 <<EOF`, `... && python3 <<EOF`, `env python3 <<EOF`,
    # `/usr/bin/python3 <<EOF`, `python3 2>/dev/null <<EOF`, hyphen-delim
    # `<<'END-MARK'`, and the here-string form `python3 <<<'code'`.
    #
    # Also walk forward through any pipes after the opener: `cat <<EOF |
    # python3 …` feeds the heredoc body into python3 just as opaquely as
    # `python3 <<EOF` does. Each pipe stage is peeled and checked for an
    # interpreter or a sourcing builtin (`eval`/`source`/`.`).
    for i, t in enumerate(tokens):
        if t not in _HEREDOC_OPENERS:
            continue
        start = i - 1
        while start >= 0 and tokens[start] not in CMD_SEPARATORS:
            start -= 1
        peeled = strip_wrappers(tokens[start + 1 : i])
        if peeled and _is_heredoc_interpreter(peeled[0]):
            return True
        j = i + 1
        while j < len(tokens):
            if tokens[j] != "|":
                j += 1
                continue
            k = j + 1
            end = k
            while end < len(tokens) and tokens[end] not in CMD_SEPARATORS:
                end += 1
            stage = strip_wrappers(tokens[k:end])
            if stage and (_is_heredoc_interpreter(stage[0]) or stage[0] in SOURCING_BUILTINS):
                return True
            j = end
    return False


def has_multiline_ansi_c(raw: str) -> bool:
    # Multi-line means either a real newline character or the `\n` escape
    # sequence inside the ANSI-C body.
    for m in ANSI_C_RE.finditer(raw):
        body = m.group("body")
        if "\n" in body or "\\n" in body:
            return True
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
        tokens = []

    if tokens:
        # body_stripped removes heredoc body tokens so script content inside a
        # `cat <<EOF` body doesn't trip any of the rules. All structural rules
        # then work off this view, walking each simple-command position (so
        # `if true; then eval …; fi` and `(bash -c …)` deny like the leading
        # form does).
        body_stripped = strip_heredoc_bodies_from_tokens(tokens)
        if has_shell_dash_c(body_stripped):
            emit_deny(REASON_BASH_C)
            return
        if has_interpreter_dash_c(body_stripped):
            emit_deny(REASON_INTERP_C)
            return
        if has_sourcing_call(body_stripped):
            emit_deny(REASON_EVAL_SOURCE)
            return
        if has_function_def(body_stripped):
            emit_deny(REASON_FUNC)
            return
        if has_loop(body_stripped):
            emit_deny(REASON_LOOP)
            return
        if has_interpreter_heredoc_tokens(tokens):
            emit_deny(REASON_HEREDOC)
            return
    elif HEREDOC_INTERP_FALLBACK_RE.search(cmd):
        # Shlex couldn't tokenise — fall back to a loose raw-text check so an
        # unparseable command with an obvious interpreter-heredoc shape still
        # denies.
        emit_deny(REASON_HEREDOC)
        return

    if has_multiline_ansi_c(cmd):
        emit_deny(REASON_ANSI_C)
        return


if __name__ == "__main__":
    main()
