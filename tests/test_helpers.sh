assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$expected" != "$actual" ]]; then
    printf '[%s] expected: %s\n             actual:   %s\n' "$label" "$expected" "$actual" >&2
    exit 1
  fi
}

assert_contains() {
  local label="$1"
  local needle="$2"
  local haystack="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    printf '[%s] Expected to find "%s" in:\n%s\n' "$label" "$needle" "$haystack" >&2
    exit 1
  fi
}
