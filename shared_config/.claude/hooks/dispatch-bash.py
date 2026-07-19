#!/usr/bin/env python3
# PreToolUse dispatcher for the Bash matcher.
#
# Runs every Bash security hook inside ONE python process instead of spawning
# one python per hook. Same rules, same order, same decisions -- only the
# process count changes (was ~10 spawns per Bash call, now 1). See AGENTS.md
# for what each underlying hook enforces.
#
# Each hook is a standalone script whose main() reads a PreToolUse payload from
# stdin and writes a permission decision (or nothing) to stdout. We import each
# once, then drive its main() in-process by swapping sys.stdin / sys.stdout
# around the call. The hooks stay byte-identical, so their own *_test.sh files
# still exercise them directly and remain the source of truth for each rule.
#
# Decision precedence across hooks is deny > ask > allow, matching Claude Code's
# own multi-hook merge (any deny blocks). This is why we run ALL applicable
# hooks rather than stopping at the first: `gh api /x/$(whoami)` must deny (the
# command-substitution hook) even though the gh guard would allow the read.
# The allow/ask hooks (gh-api-write-guard, cd-under-roots, file-ops-under-roots,
# deny-bash-script, bash-allow-trusted) still work -- an allow only wins when no
# hook denied or asked. The first hook to reach the winning level supplies the
# reason.

import importlib.util
import io
import json
import os
import re
import shlex
import sys

HOOK_DIR = os.path.dirname(os.path.abspath(__file__))


def _lead_tokens(cmd):
    # Tokenize the way the hooks do, so a guard sees the same leading token the
    # hook would. Returns [] when the command can't be lexed.
    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        return list(lex)
    except ValueError:
        return []


def _is_gh_api(cmd):
    s = cmd.lstrip()
    return s.startswith("gh api ") or s.startswith("/opt/homebrew/bin/gh api ")


def _lead_in(names):
    def guard(cmd):
        toks = _lead_tokens(cmd)
        return bool(toks) and toks[0] in names
    return guard


# Ordered exactly as the Bash matcher was wired in settings.json. A guard of
# None means the hook ran unconditionally there; a callable mirrors its `if`.
# The gh/cd/git hooks also self-guard on their leading token, so their guard is
# belt-and-suspenders -- but deny-unallowlisted-host scans any command for
# hosts and does NOT self-guard, so its curl/wget guard is required to avoid
# denying an unrelated command that merely mentions a URL.
HOOKS = [
    ("gh-api-write-guard.py", _is_gh_api),
    ("cd-under-roots.py", _lead_in({"cd"})),
    ("git-deny-dash-c.py", _lead_in({"git", "/usr/bin/git"})),
    ("file-ops-under-roots.py", None),
    ("deny-json-tool.py", None),
    ("deny-awk.py", None),
    ("deny-find-root.py", None),
    ("deny-multi-command.py", None),
    ("deny-shell-wrapper.py", None),
    ("deny-command-substitution.py", None),
    ("deny-bash-script.py", None),
    ("deny-env-vars.py", None),
    ("deny-unallowlisted-host.py",
     _lead_in({"curl", "/usr/bin/curl", "wget", "/usr/bin/wget"})),
    ("bash-allow-trusted.py", None),
]

RANK = {"deny": 3, "ask": 2, "allow": 1}


def _skipped():
    # Hook filenames to skip for this session, from CLAUDE_BASH_HOOKS_SKIP
    # (comma or space separated, `.py` optional), e.g.
    # CLAUDE_BASH_HOOKS_SKIP="deny-awk.py deny-find-root". Hooks inherit the
    # session's env, so setting it at launch scopes the skip to one session.
    # This lowers the permission-hook safety floor. It is a one-off debugging
    # escape, not a config knob. The OS sandbox is a separate layer and stays on.
    out = set()
    raw = os.environ.get("CLAUDE_BASH_HOOKS_SKIP", "")
    for tok in re.split(r"[,\s]+", raw.strip()):
        if tok:
            out.add(tok if tok.endswith(".py") else tok + ".py")
    return out


def _load(fname):
    path = os.path.join(HOOK_DIR, fname)
    spec = importlib.util.spec_from_file_location(
        "hook_" + re.sub(r"\W", "_", fname), path
    )
    mod = importlib.util.module_from_spec(spec)
    # exec_module runs the hook's top level (defs, module constants) but not its
    # main(): the module name is not "__main__". __file__ is set to the real
    # path, so hooks that locate config via realpath(__file__) still find it.
    spec.loader.exec_module(mod)
    return mod


def _load_all():
    mods, failed = {}, []
    for fname, _ in HOOKS:
        try:
            mods[fname] = _load(fname)
        except Exception as exc:
            failed.append(f"{fname}: {exc}")
    if failed:
        # ponytail: a hook that fails to import is skipped, which matches
        # today's behavior -- a crashing hook process also yields no decision.
        # Upgrade path if a check must fail-closed: subprocess-fallback it.
        sys.stderr.write(
            "dispatch-bash: SKIPPING hooks that failed to load: "
            + "; ".join(failed) + "\n"
        )
    return mods


def _run(mod, payload):
    # Drive the hook's main() with the payload on stdin, capturing its stdout.
    buf = io.StringIO()
    old_in, old_out = sys.stdin, sys.stdout
    sys.stdin, sys.stdout = io.StringIO(payload), buf
    try:
        mod.main()
    except SystemExit:
        pass
    except Exception:
        return ""  # a crashing hook yields no decision, as before
    finally:
        sys.stdin, sys.stdout = old_in, old_out
    return buf.getvalue()


def main():
    payload = sys.stdin.read()
    try:
        cmd = (json.loads(payload).get("tool_input") or {}).get("command") or ""
    except Exception:
        return  # unparseable payload -> stay silent, like the hooks do
    if not cmd:
        return

    skip = _skipped()
    if skip:
        sys.stderr.write(
            "dispatch-bash: SKIPPING hooks this session: "
            + ", ".join(sorted(skip)) + "\n"
        )

    mods = _load_all()
    best = None  # (rank, order_index, raw_output)
    for i, (fname, guard) in enumerate(HOOKS):
        if fname in skip:
            continue
        if guard and not guard(cmd):
            continue
        mod = mods.get(fname)
        if mod is None:
            continue
        raw = _run(mod, payload)
        if not raw.strip():
            continue
        try:
            decision = json.loads(raw)["hookSpecificOutput"]["permissionDecision"]
        except Exception:
            continue
        rank = RANK.get(decision, 0)
        # Strictly-greater keeps the FIRST hook at the winning level, so its
        # reason is the one shown.
        if best is None or rank > best[0]:
            best = (rank, i, raw)

    if best is not None:
        sys.stdout.write(best[2])


if __name__ == "__main__":
    main()
