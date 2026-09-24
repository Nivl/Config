#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/ask-prose-claims.py against a fixture
# repo: feeds it PreToolUse Bash payloads with a cwd and asserts the decision.
#   git commit with an unnamed quantifier in staged prose      -> deny
#   git commit with an unresolved pointer in a staged comment  -> deny
#   quantifier in the -m / heredoc commit message              -> deny
#   quantifier in a CODE line, or inside backticks, or a URL   -> silent
#   PROSE_CLAIMS_OK=1 prefix on a commit with hits              -> silent (falls to the human prompt)
#   pointer that resolves (repo-relative, ~/, absolute)        -> silent
#   not a git commit (git status, git commit as a -m value)    -> silent
#   no staged changes and a clean message                      -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/ask-prose-claims.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/ask-prose-claims-fixture-$$"
rm -rf "$FIX"
mkdir -p "$FIX/src" "$FIX/docs"
cd "$FIX"
git init -q .
git config user.email t@example.com
git config user.name t
printf 'export const a = 1\n' > src/a.ts
printf '# notes\n' > docs/notes.md
git add -A
git commit -qm init

decision() {
  local payload out
  payload="$(jq -nc --arg c "$1" --arg d "$FIX" '{cwd: $d, tool_input: {command: $c}}')"
  out="$(printf '%s' "$payload" | python3 "$HOOK")"
  if [[ -z "$out" ]]; then echo "silent"; else jq -r '.hookSpecificOutput.permissionDecision' <<<"$out"; fi
}

reason() {
  local payload
  payload="$(jq -nc --arg c "$1" --arg d "$FIX" '{cwd: $d, tool_input: {command: $c}}')"
  printf '%s' "$payload" | python3 "$HOOK" | jq -r '.hookSpecificOutput.permissionDecisionReason'
}

stage() { printf '%s\n' "$2" >> "$1"; git add "$1"; }
unstage_all() { git reset -q --hard HEAD; }

# ---- Silent: not a commit, or nothing staged ----
assert_eq "silent_status" "silent" "$(decision 'git status')"
assert_eq "silent_clean_commit" "silent" "$(decision 'git commit -m "add a thing"')"

# ---- Quantifier in staged prose (markdown) -> deny ----
stage docs/notes.md 'This helper is the only reader of the flag.'
assert_eq "deny_md_quantifier" "deny" "$(decision 'git commit -m "docs"')"
assert_contains "reason_names_file_line" "docs/notes.md:2: \`only\`" "$(reason 'git commit -m "docs"')"
unstage_all

# ---- Quantifier in a TS comment -> deny; in TS code -> silent ----
stage src/a.ts '// every caller checks the flag first'
assert_eq "deny_ts_comment" "deny" "$(decision 'git commit -m x')"
unstage_all
stage src/a.ts 'const only = never(always)'
assert_eq "silent_ts_code" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Backticks and URLs are literal content -> silent ----
stage docs/notes.md 'Set `only` to skip. See https://example.com/never/always'
assert_eq "silent_literal" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Quantifier in the commit message -> deny (dash-m and heredoc) ----
assert_eq "deny_message_m" "deny" "$(decision 'git commit -m "this is the only fix"')"
assert_eq "deny_message_am" "deny" "$(decision 'git commit -am "this is the only fix"')"
assert_eq "deny_message_qm" "deny" "$(decision 'git commit -qm "nothing else reads it"')"
printf -v HEREDOC "git commit -q -F - <<'EOF'\nfix: thing\n\nNothing else reads it.\nEOF"
assert_eq "deny_message_heredoc" "deny" "$(decision "$HEREDOC")"
assert_contains "reason_message_line" "commit message line 3" "$(reason "$HEREDOC")"
printf -v CLEAN "git commit -q -F - <<'EOF'\nfix: thing\n\nRead by src/a.ts.\nEOF"
assert_eq "silent_message_clean" "silent" "$(decision "$CLEAN")"
printf -v OTHERDOC "python3 - <<'EOF'\nprint('only never always')\nEOF\ngit commit -q -F - <<'MSG'\nfix: thing\nMSG"
assert_eq "silent_other_heredoc" "silent" "$(decision "$OTHERDOC")"

# ---- Pointers ----
stage src/a.ts '// see src/missing.ts:12 for the guard'
assert_eq "deny_pointer_missing" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_pointer" "\`src/missing.ts:12\` does not resolve" "$(reason 'git commit -m x')"
unstage_all
stage src/a.ts '// see docs/notes.md and src/a.ts:1'
assert_eq "silent_pointer_resolves" "silent" "$(decision 'git commit -m x')"
unstage_all
stage docs/notes.md "Absolute $FIX/src/a.ts and missing $FIX/src/nope.ts"
assert_eq "deny_pointer_abs_missing" "deny" "$(decision 'git commit -m x')"
unstage_all
stage docs/notes.md 'Pointer with a version 1.2.3 and a bare package.json are not pointers'
assert_eq "silent_not_pointers" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Comment naming a symbol the file's code lacks -> deny; one the code names -> silent ----
printf 'export const paymentStatus = 1\nfunction refreshCache() {}\n' > src/a.ts
git add src/a.ts && git commit -qm base
stage src/a.ts '// updateSubscriptionWithNewData would mail the subscriber here'
assert_eq "deny_ident_missing" "deny" "$(decision 'git commit -m x')"
SUBDIR_OUT="$(jq -nc --arg c 'git commit -m x' --arg d "$FIX/src" '{cwd: $d, tool_input: {command: $c}}' | python3 "$HOOK" | jq -r '.hookSpecificOutput.permissionDecision')"
assert_eq "deny_ident_from_subdir" "deny" "$SUBDIR_OUT"
assert_contains "reason_ident" "\`updateSubscriptionWithNewData\` is named by this comment and by no code line" "$(reason 'git commit -m x')"
unstage_all
stage src/a.ts '// payment_status is read after refreshCache runs, see calm_model.find_all'
assert_eq "deny_ident_dotted_missing" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_ident_norm_ok" "calm_model.find_all" "$(reason 'git commit -m x')"
unstage_all
stage src/a.ts '// payment_status is read after refreshCache runs'
assert_eq "silent_ident_normalised" "silent" "$(decision 'git commit -m x')"
unstage_all
printf 'const total = base\n  * 2\n  * scaleFactor;\n// scaleFactor doubles the result\n' > src/star.ts
git add src/star.ts
assert_eq "silent_star_continuation" "silent" "$(decision 'git commit -m x')"
unstage_all
stage src/a.ts '// JSON.parse and Promise.all and console.log and useEffect are fine here'
assert_eq "silent_builtins" "silent" "$(decision 'git commit -m x')"
unstage_all
printf '/**\n * Doc block naming onlyInComment.\n */\nexport const real = 1\n// onlyInComment is not code\n' > src/blk.ts
git add src/blk.ts
assert_eq "deny_block_comment_not_code" "deny" "$(decision 'git commit -m x')"
unstage_all
stage src/a.ts '// e.g. a status from calm.com, i.e. the mirror row'
assert_eq "silent_ident_abbrev" "silent" "$(decision 'git commit -m x')"
unstage_all
stage docs/notes.md 'Markdown may name updateSubscriptionWithNewData freely'
assert_eq "silent_ident_markdown" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Comment block length ----
for i in 1 2 3 4 5 6 7 8 9; do printf '// line %s of a comment block\n' "$i" >> src/a.ts; done
git add src/a.ts
assert_eq "deny_long_block" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_long_block" "added comment block of 9 lines" "$(reason 'git commit -m x')"
unstage_all
for i in 1 2 3 4 5 6 7 8; do printf '// line %s of a comment block\n' "$i" >> src/a.ts; done
git add src/a.ts
assert_eq "silent_block_at_limit" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- An added line that itself starts with ++ is content, not a file header ----
printf '++ not a header\nThis helper is the only reader.\n' > docs/patch.md
git add docs/patch.md
assert_eq "deny_plusplus_content" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_plusplus_path_kept" "docs/patch.md:2" "$(reason 'git commit -m x')"
unstage_all

# ---- Hit cap ----
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do printf 'only %s\n' "$i" >> docs/notes.md; done
git add docs/notes.md
assert_contains "reason_capped" "... and 2 more" "$(reason 'git commit -m x')"
unstage_all

# ---- Chained, multi-line and env-prefixed forms still seen; commit as a value is not ----
stage docs/notes.md 'always'
assert_eq "deny_chained" "deny" "$(decision 'git add -A && git commit -m x')"
printf -v MULTI "echo hi\ngit commit -m x"
assert_eq "deny_multiline" "deny" "$(decision "$MULTI")"
assert_eq "deny_env_prefix" "deny" "$(decision 'GIT_AUTHOR_NAME=x git commit -m x')"
assert_eq "silent_override" "silent" "$(decision 'PROSE_CLAIMS_OK=1 git commit -m x')"
assert_contains "reason_names_override" "PROSE_CLAIMS_OK=1" "$(reason 'git commit -m x')"
assert_eq "silent_commit_as_value" "silent" "$(decision 'echo commit')"
assert_eq "silent_stash" "silent" "$(decision 'git stash')"
assert_eq "silent_gh" "silent" "$(decision 'gh pr create --title commit')"
unstage_all

# ---- Scheme-less web paths are not pointers ----
stage docs/notes.md 'See example.com/docs/guide.md for details.'
assert_eq "silent_bare_domain" "silent" "$(decision 'git commit -m x')"
unstage_all

# ---- Non-ASCII punctuation in added prose and the message ----
stage src/a.ts '// retry once — the lock may be held'
assert_eq "deny_emdash_comment" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_emdash_sub" "src/a.ts:3: — -> two sentences" "$(reason 'git commit -m x')"
unstage_all
stage docs/notes.md 'Step one → step two, and “quoted”.'
assert_contains "reason_arrow_sub" "→ -> ->" "$(reason 'git commit -m x')"
assert_contains "reason_curly_sub" "“ -> \"" "$(reason 'git commit -m x')"
unstage_all
stage docs/notes.md 'The literal `a → b` is quoted output.'
assert_eq "silent_glyph_in_backticks" "silent" "$(decision 'git commit -m x')"
unstage_all
stage src/a.ts "const arrow = '→'"
assert_eq "silent_glyph_in_code" "silent" "$(decision 'git commit -m x')"
unstage_all
assert_eq "deny_glyph_in_message" "deny" "$(decision 'git commit -m "fix: a → b"')"

cd /
rm -rf "$FIX"
echo "ask-prose-claims.py: all tests passed"
