#!/usr/bin/env python3
# PermissionDenied / PostToolUseFailure hook for WebFetch: when a fetch
# is denied (declined domain prompt, deny rule) or fails (network /
# sandbox block / 4xx-5xx), inject a steer telling the agent to STOP,
# report the unreachable domain to the user, and ask how to proceed —
# instead of quietly retrying, falling back to curl, hitting another
# source, or web-searching around the block.
#
# Why a hook and not only an AGENTS.md rule: this fires for the tool
# call regardless of which agent ran it, so subagents (which may not
# load CLAUDE.md) are covered too. It steers via additionalContext —
# advice injected into the agent's context, not a hard stop.
#
# Registered (see settings.json) on both PermissionDenied and
# PostToolUseFailure with matcher "WebFetch". The output echoes the
# input's hook_event_name so additionalContext is attributed to the
# event that actually fired.

import json
import sys
from urllib.parse import urlparse


def _target(tool_input):
    # Name the host so the message is concrete and the allowlist hint is
    # copy-pasteable. Fall back to the raw url, then to nothing.
    url = (tool_input.get("url") or "").strip()
    if not url:
        return ""
    host = urlparse(url).netloc or url
    return host.split("@")[-1].split(":")[0].strip().rstrip(".")


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        return
    event = data.get("hook_event_name") or "PostToolUseFailure"
    tool = data.get("tool_name") or "the fetch tool"
    target = _target(data.get("tool_input") or {})
    where = f"`{target}`" if target else f"the target ({tool})"
    hint = (
        f" If the user wants it allowlisted, the fix is "
        f"`melvin-config claude perms allow add --fetch {target}` "
        "(then resync + restart)."
        if target
        else ""
    )
    msg = (
        f"{tool} could not access {where}. Do NOT work around this: no "
        "curl/wget, no alternate source, no web search, no guessing the "
        "content. Stop now, tell the user you could not reach "
        f"{where}, and ask how they want to proceed.{hint}"
    )
    json.dump(
        {"hookSpecificOutput": {
            "hookEventName": event,
            "additionalContext": msg,
        }},
        sys.stdout,
    )


if __name__ == "__main__":
    main()
