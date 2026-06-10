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

echo "settings schema: all tests passed"
