#!/usr/bin/env bash
# Verifies dispatch-bash.py reproduces exactly what the individual Bash hooks
# produce when run separately. For each command we compute the "ground truth"
# by running every applicable hook as its own subprocess and merging with the
# deny > ask > allow precedence, then assert the single-process dispatcher
# returns the same decision. This catches stdin/stdout-swap, ordering, guard,
# and module-load bugs without hard-coding per-command expectations.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOKS="$SCRIPT_DIR/shared_config/.claude/hooks"
DISPATCH="$HOOKS/dispatch-bash.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# Same order and guards as dispatch-bash.py's HOOKS list.
HOOK_SPECS=(
  "gh-api-write-guard.py:gh"
  "cd-under-roots.py:cd"
  "git-deny-dash-c.py:git"
  "file-ops-under-roots.py:none"
  "deny-json-tool.py:none"
  "deny-awk.py:none"
  "deny-find-root.py:none"
  "deny-multi-command.py:none"
  "deny-shell-wrapper.py:none"
  "deny-command-substitution.py:none"
  "deny-bash-script.py:none"
  "deny-env-vars.py:none"
  "deny-unallowlisted-host.py:net"
  "bash-allow-trusted.py:none"
)

lead() { printf '%s' "$1" | awk '{print $1; exit}'; }

guard_ok() { # $1=guard $2=cmd
  local g="$1" c="$2" first
  case "$g" in
    none) return 0 ;;
    cd)   [[ "$(lead "$c")" == "cd" ]] ;;
    git)  first="$(lead "$c")"; [[ "$first" == "git" || "$first" == "/usr/bin/git" ]] ;;
    gh)   [[ "$c" == "gh api "* || "$c" == "/opt/homebrew/bin/gh api "* ]] ;;
    net)  first="$(lead "$c")"
          [[ "$first" == curl || "$first" == wget || "$first" == /usr/bin/curl || "$first" == /usr/bin/wget ]] ;;
  esac
}

decision_of() { # $1=hook $2=payload -> decision or ""
  local out
  out="$(printf '%s' "$2" | python3 "$HOOKS/$1")" || true
  if [[ -z "$out" ]]; then echo ""; else jq -r '.hookSpecificOutput.permissionDecision // ""' <<<"$out"; fi
}

merged() { # $1=cmd -> merged decision
  local cmd="$1" payload best=0 bestdec="silent" spec hook g dec rank
  payload="$(jq -nc --arg c "$cmd" '{tool_input:{command:$c}}')"
  for spec in "${HOOK_SPECS[@]}"; do
    hook="${spec%%:*}"; g="${spec##*:}"
    guard_ok "$g" "$cmd" || continue
    dec="$(decision_of "$hook" "$payload")"
    case "$dec" in deny) rank=3 ;; ask) rank=2 ;; allow) rank=1 ;; *) rank=0 ;; esac
    if (( rank > best )); then best=$rank; bestdec="${dec:-silent}"; fi
  done
  echo "$bestdec"
}

dispatch_decision() { # $1=cmd
  local payload out
  payload="$(jq -nc --arg c "$1" '{tool_input:{command:$c}}')"
  out="$(printf '%s' "$payload" | python3 "$DISPATCH")" || true
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision // "silent"' <<<"$out"; fi
}

CMDS=(
  "ls -la"
  "find /"
  "find /etc -name hosts"
  "echo \$(whoami)"
  "echo \$TMPDIR"
  "git -C /tmp status"
  "git status"
  "gh api /user"
  "gh api -X POST /repos/o/r/issues -f title=x"
  "gh api /user/\$(whoami)"
  "rm /etc/hosts"
  "rm /tmp/foo"
  "awk '{print}' /etc/hosts"
  "echo hi && echo bye"
  ""
)

for c in "${CMDS[@]}"; do
  exp="$(merged "$c")"
  act="$(dispatch_decision "$c")"
  assert_eq "dispatch matches merged for [$c]" "$exp" "$act"
done

# The precedence regression, asserted explicitly: gh's read-only allow must not
# short-circuit the command-substitution deny.
assert_eq "gh_api_cmdsub_denies" "deny" "$(dispatch_decision "gh api /user/\$(whoami)")"

echo "dispatch-bash.py: all tests passed"
