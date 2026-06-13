#!/bin/zsh
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
# Optional positional arg so a fixture can be checked instead of the
# real file (used for negative-testing the assertions themselves).
SETTINGS="${1:-$SCRIPT_DIR/shared_config/.claude/settings.json}"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

# Claude Code only honors these keys nested under .permissions or
# .sandbox. At the top level they validate fine (the settings schema
# tolerates unknown root keys) but are silently ignored — the file
# looks configured while doing nothing. This happened to
# additionalDirectories, which sat inert at the root for a week.
jq empty "$SETTINGS"

for key in additionalDirectories allow ask deny defaultMode \
  excludedCommands autoAllowBashIfSandboxed filesystem network; do
  assert_eq "no_top_level_$key" "false" \
    "$(jq --arg k "$key" 'has($k)' "$SETTINGS")"
done

assert_eq "additional_dirs_under_permissions" "true" \
  "$(jq '.permissions | has("additionalDirectories")' "$SETTINGS")"

# Relative entries would silently scope to wherever the session starts.
assert_eq "additional_dirs_absolute" "true" \
  "$(jq '.permissions.additionalDirectories | (length > 0) and all(startswith("/") or startswith("~"))' "$SETTINGS")"

# Every registered hook command must exist (and be executable) in this
# checkout. A renamed hook with a stale registration fails OPEN at
# runtime: the spawn failure is non-blocking and the guard silently
# vanishes. Registrations store the deploy-time absolute path, so
# re-anchor each on the shared_config/ segment to map it back to this
# checkout — independent of the deploy prefix (user, home dir, CI
# runner). read line-by-line so a path with spaces stays one entry.
while IFS= read -r hook_cmd; do
  rel="${hook_cmd##*/shared_config/}"
  if [[ "$rel" == "$hook_cmd" ]]; then
    printf '[hook_path] registered hook is not under shared_config/: %s\n' "$hook_cmd" >&2
    exit 1
  fi
  local_path="$SCRIPT_DIR/shared_config/$rel"
  if [[ ! -x "$local_path" ]]; then
    printf '[hook_path] registered hook missing or not executable: %s (checked %s)\n' \
      "$hook_cmd" "$local_path" >&2
    exit 1
  fi
done < <(jq -r '.hooks[][] | .hooks[]? | select(.type == "command") | .command' "$SETTINGS")

# Negative self-test: a registered hook pointing at a missing file must
# make the check above exit non-zero (the fail-open scenario it exists
# to catch). Guarded on no-arg so the fixture re-run doesn't recurse.
if [[ $# -eq 0 ]]; then
  neg_fixture="$(mktemp /tmp/settings_schema_neg.XXXXXX)"
  jq '.hooks.PreToolUse[0].hooks[0].command =
        "/Users/melvin/.melvin/config/shared_config/.claude/hooks/__missing__.py"' \
    "$SETTINGS" >"$neg_fixture"
  if bash "$0" "$neg_fixture" >/dev/null 2>&1; then
    rm -f "$neg_fixture"
    printf '[hook_path_negative] a missing hook path did not fail the check\n' >&2
    exit 1
  fi
  rm -f "$neg_fixture"
fi

echo "settings schema: all tests passed"
