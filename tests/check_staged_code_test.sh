#!/usr/bin/env bash
# Exercises shared_config/.claude/hooks/check-staged-code.py against a fixture repo.
#   removed symbol still named in markdown or a comment    -> deny
#   removed symbol still called elsewhere, or CHANGELOG     -> silent
#   added test with only absence / bare-throw assertions   -> deny
#   added test with a positive assertion, or none           -> silent
#   `as unknown as` in production code                      -> deny
#   the same in a test file, a string, or a comment         -> silent
#   CODE_CHECKS_OK=1, or not a commit                       -> silent
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
HOOK="$SCRIPT_DIR/shared_config/.claude/hooks/check-staged-code.py"

. "$(cd "$(dirname "$0")" && pwd)/test_helpers.sh"

FIX="/tmp/claude/check-staged-code-fixture-$$"
rm -rf "$FIX"
mkdir -p "$FIX/src" "$FIX/docs" "$FIX/test"
cd "$FIX"
git init -q .
git config user.email t@example.com
git config user.name t
printf 'export function chargeCustomerNow() {}\nexport function refundCustomerLater() {}\n' > src/billing.ts
printf 'export const k = 1\n' > src/a.ts
printf 'import { refundCustomerLater } from "./billing"\nrefundCustomerLater()\n' > src/caller.ts
printf '# Billing\n\n`chargeCustomerNow` charges at once.\n' > docs/map.md
printf 'export const x = 1\n' > test/foo.test.ts
git add -A
git commit -qm init
trap 'cd /; rm -rf "$FIX"' EXIT

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
reset() { git reset -q --hard HEAD; }

assert_eq "silent_status" "silent" "$(decision 'git status')"
assert_eq "silent_nothing_staged" "silent" "$(decision 'git commit -m x')"

# ---- Dead reference ----
printf 'export function refundCustomerLater() {}\n' > src/billing.ts
git add -A
assert_eq "deny_dead_ref_md" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_dead_ref_where" "\`chargeCustomerNow\` is gone from every code line in the index but still named at docs/map.md:3" "$(reason 'git commit -m x')"
assert_eq "silent_override" "silent" "$(decision 'CODE_CHECKS_OK=1 git commit -m x')"
printf '# Billing\n' > docs/map.md
git add -A
assert_eq "silent_doc_updated_too" "silent" "$(decision 'git commit -m x')"
reset

printf '// chargeCustomerNow used to live here\nexport const k = 1\n' > src/a.ts
printf '# Billing\n' > docs/map.md
git add -A
git commit -qm comment
printf 'export function refundCustomerLater() {}\n' > src/billing.ts
git add -A
assert_eq "deny_dead_ref_comment" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_dead_ref_comment" "src/a.ts:1" "$(reason 'git commit -m x')"
reset
git reset -q --hard HEAD~1

printf 'export function chargeCustomerNow() {}\n' > src/billing.ts
git add -A
assert_eq "silent_removed_but_still_called" "silent" "$(decision 'git commit -m x')"
reset

printf '{ "handler": "chargeCustomerNow" }\n' > config.json
git add -A
git commit -qm json
printf 'export function refundCustomerLater() {}\n' > src/billing.ts
git add -A
assert_eq "silent_still_named_in_json" "silent" "$(decision 'git commit -m x')"
reset
git reset -q --hard HEAD~1

printf '# Billing\n' > docs/map.md
printf '# Changes\n\n- chargeCustomerNow removed\n' > CHANGELOG.md
git add -A
git commit -qm changelog
printf 'export function refundCustomerLater() {}\n' > src/billing.ts
git add -A
assert_eq "silent_changelog_only" "silent" "$(decision 'git commit -m x')"
reset
git reset -q --hard HEAD~1

mkdir -p docs/adr
printf '# Billing\n' > docs/map.md
printf '# 0001\n\nWe chose `chargeCustomerNow`.\n' > docs/adr/0001-billing.md
git add -A
git commit -qm adr
printf 'export function refundCustomerLater() {}\n' > src/billing.ts
git add -A
assert_eq "silent_adr_only" "silent" "$(decision 'git commit -m x')"
reset
git reset -q --hard HEAD~1

# ---- Vacuous tests ----
cat > test/foo.test.ts <<'EOF'
export const x = 1
describe('refund', () => {
  it('sends no email when the charge failed', () => {
    run()
    expect(sendEmail).to.not.have.been.called
    expect(chargeStub.callCount).to.equal(0)
  })
})
EOF
git add -A
assert_eq "deny_absence_only" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_vacuous_names_test" "test/foo.test.ts:3: test \"sends no email when the charge failed\" has 2 assertion(s)" "$(reason 'git commit -m x')"
reset

cat > test/foo.test.ts <<'EOF'
export const x = 1
it('rejects a bad payload', () => {
  expect(() => parse('x')).to.throw()
})
EOF
git add -A
assert_eq "deny_bare_throw" "deny" "$(decision 'git commit -m x')"
reset

printf "export const x = 1\nit('sends nothing', () => { expect(send).to.not.have.been.called })\n" > test/foo.test.ts
git add -A
assert_eq "deny_one_line_test" "deny" "$(decision 'git commit -m x')"
reset

cat > test/foo.test.ts <<'EOF'
export const x = 1
it('returns the total', () => {
  expect(result).to.exist.and.to.equal(42)
})
EOF
git add -A
assert_eq "silent_chained_positive" "silent" "$(decision 'git commit -m x')"
reset

cat > test/foo.test.ts <<'EOF'
export const x = 1
it('refunds once', () => {
  expect(sendEmail).to.not.have.been.called
  expect(refund).to.have.been.calledOnceWith('ch_1')
})
EOF
git add -A
assert_eq "silent_has_positive" "silent" "$(decision 'git commit -m x')"
reset

cat > test/foo.test.ts <<'EOF'
export const x = 1
it('leaves the description undefined when none is given', () => {
  expect(build({}).description).to.be.undefined
})
EOF
git add -A
assert_eq "silent_named_absence_value" "silent" "$(decision 'git commit -m x')"
reset

cat > test/foo.test.ts <<'EOF'
export const x = 1
it('runs the shared contract', () => {
  runContract(subject)
})
EOF
git add -A
assert_eq "silent_no_assertions" "silent" "$(decision 'git commit -m x')"
reset

# ---- Double cast ----
printf 'export const k = 1\nconst req = raw as unknown as Request\n' > src/a.ts
git add -A
assert_eq "deny_double_cast" "deny" "$(decision 'git commit -m x')"
assert_contains "reason_double_cast" "src/a.ts:2: \`as unknown as\`" "$(reason 'git commit -m x')"
reset

printf 'export const x = 1\nconst d = { platform: 1 } as unknown as Device\n' > test/foo.test.ts
git add -A
assert_eq "silent_cast_in_test" "silent" "$(decision 'git commit -m x')"
reset

mkdir -p libraries/test-utils
printf 'export const s = {} as unknown as Sub\n' > libraries/test-utils/fixtures.ts
git add -A
assert_eq "silent_cast_in_test_utils" "silent" "$(decision 'git commit -m x')"
reset
rm -rf libraries

printf 'export const k = 1\n// we avoid as unknown as here\nconst s = "as unknown as"\n' > src/a.ts
git add -A
assert_eq "silent_cast_in_comment_or_string" "silent" "$(decision 'git commit -m x')"
reset

# ---- The helper module is missing: silent, exit 0 ----
LONE="/tmp/claude/check-staged-code-lone-$$"
mkdir -p "$LONE"
cp "$HOOK" "$LONE/check-staged-code.py"
printf 'export const k = 1\nconst req = raw as unknown as Request\n' > src/a.ts
git add -A
OUT="$(jq -nc --arg d "$FIX" '{cwd: $d, tool_input: {command: "git commit -m x"}}' | python3 "$LONE/check-staged-code.py" 2>&1; echo "exit=$?")"
assert_eq "silent_without_helper" "exit=0" "$OUT"
rm -rf "$LONE"
reset

echo "check-staged-code.py: all tests passed"
