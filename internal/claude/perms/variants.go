// Package perms manages the permissions section of
// shared_config/.claude/settings.json. It exposes a small surface
// (Variants, Load/Save, Add/Remove) used by the `melvin-config claude
// perms` command tree to add and remove allow/ask/deny rules with
// stable alphabetical ordering and cross-list conflict detection.
package perms

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"
)

// lookPath resolves a bash command's leading token to an absolute path
// (via exec.LookPath). Held in a package var so tests in this package
// — and in callers like the cmd package — can substitute a
// deterministic stub via SetLookPath. Production code never reads or
// writes this var directly.
var lookPath = exec.LookPath //nolint:gochecknoglobals // testing seam, swapped via SetLookPath

// SetLookPath replaces the PATH-resolution function used by Variants
// and returns a closure that restores the previous one. Intended for
// tests in other packages that need deterministic Bash() rule
// expansion regardless of the test machine's PATH; production code
// must not call it.
func SetLookPath(f func(string) (string, error)) (restore func()) {
	prev := lookPath
	lookPath = f
	return func() { lookPath = prev }
}

// Kind selects which rule-shape Variants expands a raw value into.
// The four kinds correspond to the four flags on `claude perms`:
// --bash, --read, --fetch, --skill.
type Kind int

const (
	// KindBash expands a value into Bash() rules. A value whose first
	// whitespace-separated token is "git" gets two variants — the raw
	// command and the `git -C /*` cwd-bypass form used when operating
	// on a worktree from a different cwd. Any other value gets just
	// the single raw variant.
	KindBash Kind = iota
	// KindRead expands a value into a single Read(<value>) rule.
	KindRead
	// KindFetch expands a value into a single
	// WebFetch(domain:<value>) rule.
	KindFetch
	// KindSkill expands a value into a single Skill(<value>) rule.
	// Values typically take the `<plugin>:<skill>` form used by
	// plugin-namespaced skills (e.g. `code-review:code-review`), but
	// the shape is opaque here — whatever string the user supplies
	// flows verbatim into the parens.
	KindSkill
)

// Variants returns the canonical permission-rule strings that should
// be added or removed for one user-supplied (kind, value) pair.
//
// Bash rules: a non-git command `X` ships as Bash(X), plus a second
// Bash(<absolute-path> <rest>) variant when the first token resolves
// via PATH (e.g. `mv *` → also `Bash(/bin/mv *)`). Git commands
// additionally get the `git -C /* <rest>` cwd-bypass form, and — when
// resolution succeeds — the absolute-path version of that form too,
// for a total of up to four rules.
//
// Path resolution is silently skipped when the command isn't found
// (shell builtins like `cd`, compound expressions, intentionally-fake
// commands) or when the resolved path equals the input first token
// (already-absolute input, e.g. `/bin/mv *`). The original Bash(value)
// is always emitted.
//
// Returned rules preserve insertion order. Upstream layers (apply.go
// and settings.go) dedupe and sort before persisting, so callers can
// rely on the slice as a stable reporting sequence even when the
// underlying expansion overlaps.
func Variants(kind Kind, value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("variants: empty value")
	}
	switch kind {
	case KindRead:
		return []string{"Read(" + value + ")"}, nil
	case KindFetch:
		return []string{"WebFetch(domain:" + value + ")"}, nil
	case KindSkill:
		return []string{"Skill(" + value + ")"}, nil
	case KindBash:
		return bashVariants(value), nil
	default:
		return nil, fmt.Errorf("variants: unknown kind %d", kind)
	}
}

// bashVariants expands a bash value into the rule list described on
// Variants. The git-detection trigger is "first whitespace-separated
// token is exactly git" — `git-foo`, `mygit`, and `cd /repo && git
// ...` all fall back to the non-git path. After the unresolved
// variants are emitted, lookPath is consulted on the first token; on
// success (path differs from the input first token), the resolved-
// path twin(s) are appended.
func bashVariants(value string) []string {
	first, rest := splitFirstToken(value)
	out := []string{
		"Bash(" + value + ")",
	}
	if first == "git" {
		out = append(out, "Bash(git -C /* "+rest+")")
	}
	if resolved, err := lookPath(first); err == nil && resolved != first {
		resolvedValue := resolved
		if rest != "" {
			resolvedValue = resolved + " " + rest
		}
		out = append(out, "Bash("+resolvedValue+")")
		if first == "git" {
			out = append(out, "Bash("+resolved+" -C /* "+rest+")")
		}
	}
	return out
}

// splitFirstToken splits s at the first run of whitespace and returns
// (first-token, rest). When s has no whitespace the rest is "".
//
// Uses unicode.IsSpace to match the upstream strings.TrimSpace in
// Variants — otherwise a value containing a control character like
// '\n' or '\v' would trim cleanly but split at the wrong boundary,
// silently falling out of the git-command detection path.
func splitFirstToken(s string) (first, rest string) {
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimLeftFunc(s[i:], unicode.IsSpace)
}
