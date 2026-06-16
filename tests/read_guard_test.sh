#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/read-guard.py: feeds it PreToolUse
# Read payloads and asserts the permission decision.
#   path under shared_config/.emacs.d/elpa -> deny (hard, any location)
#   sensitive file (.env, *.pem, ...)      -> ask
#   path under an allowed root             -> allow
#   anything else                          -> silent (default prompt)
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/read-guard.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# decision <file_path> -> permissionDecision string, or "silent" when the
# hook emits nothing, or "error:<status>" when it exits non-zero.
decision() {
  local payload out status=0
  payload="$(jq -nc --arg p "$1" '{cwd: "/tmp", tool_input: {file_path: $p}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")" || status=$?
  if ((status != 0)); then
    echo "error:$status"
  elif [[ -z "$out" ]]; then
    echo "silent"
  else
    jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"
  fi
}

# ---- Deny: the elpa package tree, wherever it lives ----
assert_eq "deny_elpa_file" "deny" \
  "$(decision "$SCRIPT_DIR/shared_config/.emacs.d/elpa/magit/magit.el")"
assert_eq "deny_elpa_dir" "deny" \
  "$(decision "$SCRIPT_DIR/shared_config/.emacs.d/elpa")"
# Segment match, not one absolute path: a worktree checkout elsewhere is
# covered too.
assert_eq "deny_elpa_worktree" "deny" \
  "$(decision "/tmp/wt/shared_config/.emacs.d/elpa/x/y.el")"
# The elpa deny is decision 0 — it beats the sensitive-file ask.
assert_eq "deny_elpa_beats_sensitive" "deny" \
  "$(decision "/tmp/shared_config/.emacs.d/elpa/foo.pem")"

# ---- Lookalikes that are NOT the elpa segment must not be denied ----
# Same parent, sibling dir whose name only starts with "elpa".
assert_eq "allow_elpa_lookalike" "allow" \
  "$(decision "/tmp/shared_config/.emacs.d/elpa-archives/x")"
# elpa appears but not as the full segment run under shared_config.
assert_eq "allow_bare_elpa_word" "allow" "$(decision "/tmp/elpa/x")"

# ---- Existing behavior still holds ----
assert_eq "ask_env_file" "ask" "$(decision "/tmp/project/.env")"
assert_eq "allow_tmp_file" "allow" "$(decision "/tmp/notes.txt")"

echo "read-guard.py: all tests passed"
