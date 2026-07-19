#!/usr/bin/env python3
# PreToolUse hook: gate `gh api` calls by read/write semantics.
#
# Reads PreToolUse JSON from stdin. For a clean, single `gh api`
# invocation it emits a permission decision:
#   - read-only call -> "allow" (skip the prompt)
#   - write call     -> "ask"   (force a confirmation prompt)
#
# A PIPE is different: we can't reason about what a downstream stage does
# with the data, so any `gh api ... | ...` is "deny"ed with a message
# steering the agent to redirect to a file instead, e.g.
#   gh api <endpoint> > /tmp/gh-api.out   (then read that file)
# The bare redirect form has no pipe and no chain token, so the read
# classifier below auto-allows it. Denying (rather than abstaining) means
# the agent self-corrects without a human prompt.
#
# The hook is self-contained: it does NOT pair with a `Bash(gh api *)`
# ask rule. An ask rule can't be bypassed by a hook "allow", so pairing
# the two would prompt on every read. Emitting "ask" for writes here
# keeps that guard without an ask rule behind it.
#
# Anything that can't be classified cleanly stays silent and falls
# through to the normal permission flow (default-mode prompt):
#   - the command contains a non-pipe shell separator (;, &, &&, ||) —
#     we can't reason about chained or backgrounded commands. (Redirects
#     like `> file` and `2>&1` are NOT separators and stay classifiable.)
#   - the command can't be tokenized
#
# A call is treated as a WRITE when any of:
#   - explicit method flag: -X / --method = POST|PUT|PATCH|DELETE
#   - any field flag (-f / -F / --field / --raw-field) UNLESS the
#     method is explicitly -X GET (gh defaults to POST when -f present)
#   - the endpoint is `graphql` and the query document is not provably
#     read-only. GraphQL always POSTs, but the body decides the real
#     semantics: a document whose top-level definitions are all `query`
#     or `fragment` (or the anonymous `{ ... }` shorthand) reads data.
#     Mutations, subscriptions, bodies we can't extract (--input,
#     @file, packed flags) or can't parse all count as writes.

import json
import re
import shlex
import sys

WRITE_METHODS = {"POST", "PUT", "PATCH", "DELETE"}
FIELD_FLAGS = {"-f", "-F", "--field", "--raw-field"}
# A pipe -> deny (steer to a file redirect). The rest -> abstain.
PIPE_TOKENS = {"|", "|&"}
CHAIN_TOKENS = {";", "&", "&&", "||"}


def graphql_doc_is_readonly(doc: str) -> bool:
    # Block strings can hide braces and keywords from the regex
    # stripping below, and they never appear at definition level in a
    # legal executable document — their presence means "can't classify".
    if '"""' in doc:
        return False
    doc = re.sub(r'"(?:\\.|[^"\\\n])*"', " ", doc)
    if '"' in doc:
        # Unterminated or multi-line string — don't guess.
        return False
    doc = re.sub(r"#[^\n\r]*", " ", doc)

    # Walk brace depth. At depth 0 the first token of each definition
    # must be `query` or `fragment`; a bare `{` is the anonymous query
    # shorthand. Anything else (mutation, subscription, junk) fails.
    # Tokens between the keyword and the body brace (operation name,
    # variable definitions, directives) are ignored.
    depth = 0
    saw_definition = False
    at_definition = True
    # Parens split off as their own tokens so `query($n: Int!)` still
    # yields a bare `query` keyword; they're otherwise ignored.
    for tok in re.findall(r"[{}()]|[^\s{}()]+", doc):
        if tok == "{":
            if depth == 0 and at_definition:
                saw_definition = True
            depth += 1
            at_definition = False
        elif tok == "}":
            depth -= 1
            if depth < 0:
                return False
            if depth == 0:
                at_definition = True
        elif depth == 0 and at_definition:
            if tok not in ("query", "fragment"):
                return False
            at_definition = False
            saw_definition = True
    return saw_definition and depth == 0


def graphql_reads_only(args: list) -> bool:
    # Collect every `query=` field body. Bail (-> write) on any form
    # whose body we can't see as a literal string in argv.
    docs = []
    i = 0
    while i < len(args):
        a = args[i]
        if a in FIELD_FLAGS:
            if i + 1 >= len(args):
                return False
            val = args[i + 1]
            i += 2
        elif a.startswith("--field=") or a.startswith("--raw-field="):
            val = a.split("=", 1)[1]
            i += 1
        elif a.startswith(("-f", "-F")) and len(a) > 2:
            # Packed short flag (-fquery=...) — don't guess its contents.
            return False
        elif a == "--input" or a.startswith("--input="):
            # Body comes from a file or stdin — can't inspect it.
            return False
        else:
            i += 1
            continue
        if val.startswith("query="):
            body = val[len("query=") :]
            if body.startswith("@"):
                # -F query=@file / @- reads the document elsewhere.
                return False
            docs.append(body)
    return bool(docs) and all(graphql_doc_is_readonly(d) for d in docs)


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


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except Exception:
        return

    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd.strip():
        return

    stripped = cmd.lstrip()
    if not (
        stripped.startswith("gh api ")
        or stripped.startswith("/opt/homebrew/bin/gh api ")
    ):
        return

    try:
        lex = shlex.shlex(cmd, posix=True, punctuation_chars=True)
        lex.whitespace_split = True
        tokens = list(lex)
    except ValueError:
        return

    if any(t in PIPE_TOKENS for t in tokens):
        emit(
            "deny",
            "gh api calls may not be piped — the read/write guard can't "
            "classify what a downstream pipeline stage does with the data. "
            "Re-run the bare call; if you need to capture or cap the output, "
            "redirect to a file instead of piping, e.g. "
            "`gh api <endpoint> > /tmp/gh-api.out`, then read that file. "
            "(Un-piped read calls are auto-allowed; write calls still prompt.)",
        )
        return

    if any(t in CHAIN_TOKENS for t in tokens):
        return

    try:
        api_idx = tokens.index("api")
    except ValueError:
        return
    args = tokens[api_idx + 1 :]
    if not args:
        return

    is_write = False
    explicit_get = False

    i = 0
    while i < len(args):
        a = args[i]
        if a in ("-X", "--method") and i + 1 < len(args):
            v = args[i + 1].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 2
            continue
        if a.startswith("-X") and len(a) > 2:
            v = a[2:].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 1
            continue
        if a.startswith("--method="):
            v = a.split("=", 1)[1].upper()
            if v in WRITE_METHODS:
                is_write = True
            elif v == "GET":
                explicit_get = True
            i += 1
            continue
        i += 1

    if args[0] == "graphql":
        # Field flags are how the query gets passed, so the generic
        # field-flag heuristic below doesn't apply — the document's own
        # operations decide read vs write.
        if not graphql_reads_only(args[1:]):
            is_write = True
    elif not explicit_get:
        for a in args:
            if a in FIELD_FLAGS:
                is_write = True
                break

    if is_write:
        emit("ask", "gh api write call — confirm before sending")
        return

    emit("allow", "gh api read-only call")


if __name__ == "__main__":
    main()
