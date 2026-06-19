#!/usr/bin/env python3
# PreToolUse hook: deny a curl/wget command whose target host is NOT on
# the network allowlist, so Claude stops and asks the user to allowlist
# the host instead of working around a silent failure.
#
# Why deny and not ask: the sandbox network layer is a binary kernel
# allow/deny — a blocked host makes the command fail mid-run with an
# opaque network error, and Claude tends to retry, cache-bust, or hunt
# for an alternate source. The allowlist also can't be granted from a
# permission prompt (it lives in settings + needs a resync), so an
# "ask" the user approves would still die at the kernel. A deny with an
# actionable reason is the only decision that produces the right
# outcome: stop, surface the blocked host, point at the fix.
#
# Scope: invoked only for curl/wget (see the `if:` entries in
# settings.json), and only http(s):// URLs in the command are checked.
# Scheme-less args (`curl example.com`) and tools that resolve a host
# from config rather than argv (`go get`, `npm`, git-over-https) are NOT
# seen here — the AGENTS.md "Blocked network hosts" rule is the backstop
# for those: on any sandbox network-deny, stop and ask to allowlist.
#
# The allowlist is read LIVE from ~/.claude (the synced settings), not
# the repo copy, so per-machine domains in settings.local.json count.
# Set MELVIN_CLAUDE_SETTINGS_DIR to point the lookup elsewhere (tests).

import json
import os
import re
import sys

# Loopback never leaves the machine, so it is never gated.
LOOPBACK = frozenset({"localhost", "127.0.0.1", "0.0.0.0", "::1"})

# Capture the whole authority (may carry userinfo + port); _host strips both.
URL_RE = re.compile(r"https?://([^/\s'\"\\?#]+)", re.IGNORECASE)


def _settings_dir():
    override = os.environ.get("MELVIN_CLAUDE_SETTINGS_DIR")
    if override:
        return override
    return os.path.expanduser("~/.claude")


def _load(path):
    try:
        with open(path, encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return {}


def _domains_from(settings):
    # WebFetch(domain:X) allow rules double as the sandbox egress
    # allowlist; sandbox.network.allowedDomains can add more.
    allowed, denied = set(), set()
    for rule in (settings.get("permissions") or {}).get("allow") or []:
        m = re.fullmatch(r"WebFetch\(domain:(.+)\)", rule.strip())
        if m:
            allowed.add(m.group(1).strip().lower())
    net = (settings.get("sandbox") or {}).get("network") or {}
    for d in net.get("allowedDomains") or []:
        allowed.add(str(d).strip().lower().lstrip("*."))
    for d in net.get("deniedDomains") or []:
        denied.add(str(d).strip().lower().lstrip("*."))
    return allowed, denied


def _allowlist():
    base = _settings_dir()
    allowed, denied = set(), set()
    for name in ("settings.json", "settings.local.json"):
        a, d = _domains_from(_load(os.path.join(base, name)))
        allowed |= a
        denied |= d
    return allowed, denied


def _matches(host, entry):
    # A bare entry covers the domain and its subdomains, mirroring how a
    # WebFetch(domain:example.com) rule also permits api.example.com.
    return host == entry or host.endswith("." + entry)


def _blocked_hosts(cmd, allowed, denied):
    out = []
    seen = set()
    for raw in URL_RE.findall(cmd):
        # Drop userinfo (before the last @) and any :port, then normalize.
        host = raw.split("@")[-1].split(":")[0].strip().lower().rstrip(".")
        if not host or host in seen or host in LOOPBACK:
            continue
        seen.add(host)
        ok = any(_matches(host, e) for e in allowed) and not any(
            _matches(host, e) for e in denied
        )
        if not ok:
            out.append(host)
    return out


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    cmd = (data.get("tool_input") or {}).get("command") or ""
    if not cmd:
        return
    allowed, denied = _allowlist()
    blocked = _blocked_hosts(cmd, allowed, denied)
    if not blocked:
        return
    host = blocked[0]
    json.dump(
        {"hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": (
                f"`{host}` is not on the network allowlist, so the sandbox "
                "will block this request. Do NOT retry, cache-bust, or find "
                "another source. Ask the user to run "
                f"`melvin-config claude perms allow add --fetch {host}` "
                "(then `melvin-config claude sync` + restart) and re-run. "
                "For reading web pages, prefer the WebFetch tool."
            ),
        }},
        sys.stdout,
    )


if __name__ == "__main__":
    main()
