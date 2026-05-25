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

// ParseGitPrefix examines a Bash command value and, when it's a git
// command in canonical hook-rule form, returns its prefix tokens —
// the subcommand path that the Python hook matches against the
// post-`git` args.
//
// Three return modes:
//
//   - non-git input (e.g. `ls *`)            → (nil, false, nil)
//   - git input in canonical form            → (prefix, true, nil)
//   - git input missing the trailing `*`     → (nil, true, error)
//
// Canonical form means: `git [subcmd-tokens...] *`, optionally
// prefixed by an absolute path to a git binary (`/usr/bin/git`,
// etc.) and/or by a `-C /*` cwd-bypass marker. Everything between
// `git` and the trailing `*` becomes the returned prefix slice.
//
// The trailing `*` is mandatory because the hook always allows
// trailing args after a matched prefix — an exact-match rule like
// `git branch --show-current` doesn't fit that shape, so we
// reject it loudly rather than silently broadening to `git branch
// --show-current <anything>`. Callers wanting exact-match
// semantics must hand-edit the hook.
func ParseGitPrefix(value string) (prefix []string, isGit bool, err error) {
	tokens := strings.Fields(strings.TrimSpace(value))
	if len(tokens) == 0 {
		return nil, false, nil
	}
	if !isGitExecutable(tokens[0]) {
		return nil, false, nil
	}
	args := tokens[1:]
	// Strip the `-C /*` cwd-bypass marker if present. This is the
	// shape produced by bashVariants() for git rules, so users who
	// paste a generated rule back through `claude perms` get the
	// same prefix as the bare form.
	if len(args) >= 2 && args[0] == "-C" && args[1] == "/*" {
		args = args[2:]
	}
	if len(args) == 0 {
		return nil, true, errors.New("git rule must include a subcommand and a trailing `*` (e.g. `git show *`)")
	}
	if args[len(args)-1] != "*" {
		return nil, true, errors.New("git rule must have a trailing `*` to be added to the hook prefix list (exact-match rules must be hand-edited in git-safe-subcommands.py)")
	}
	prefix = args[:len(args)-1]
	if len(prefix) == 0 {
		return nil, true, errors.New("git rule needs at least one subcommand token before the trailing `*`")
	}
	return prefix, true, nil
}

// isGitExecutable reports whether token is the literal `git` or an
// absolute path whose basename is `git`. The Python hook is
// stricter (an explicit KNOWN_GIT_PATHS allowlist), but for input
// canonicalization here we accept any absolute-path form — the
// resulting prefix is the same regardless of which binary path the
// user pasted.
func isGitExecutable(token string) bool {
	if token == "git" {
		return true
	}
	if !strings.HasPrefix(token, "/") {
		return false
	}
	return strings.HasSuffix(token, "/git")
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
