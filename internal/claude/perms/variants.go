// Package perms manages the permissions section of
// shared_config/.claude/settings.json. It exposes a small surface
// (Variants, Load/Save, Add/Remove) used by the `melvin-config claude
// perms` command tree to add and remove allow/ask/deny rules with
// stable alphabetical ordering and cross-list conflict detection.
package perms

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Kind selects which rule-shape Variants expands a raw value into.
// The four kinds correspond to the four flags on `claude perms`:
// --bash, --read, --fetch, --skill.
type Kind int

const (
	// KindBash expands a value into Bash() rules. A value whose first
	// whitespace-separated token is "git" gets six variants (covering
	// `git`, `git -C /*`, and the rtk/rtk-proxy wrappers of each);
	// any other value gets three (the raw command plus rtk and
	// rtk-proxy wrappers).
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
// Bash rules account for melvin's RTK proxy: any non-git bash command
// `X` ships as three rules — Bash(X), Bash(rtk X), Bash(rtk proxy X)
// — because the same operation can hit the permission engine through
// any of those entry points. Git commands additionally need the
// `git -C /* <rest>` form (used when operating on a worktree from a
// different cwd) so they get six variants total.
//
// Returned rules are deduplicated but preserve insertion order so
// callers can use the result as a stable sequence for reporting.
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

// bashVariants expands a bash value into the three- or six-rule list
// described on Variants. The git-detection trigger is "first
// whitespace-separated token is exactly git" — `git-foo`, `mygit`,
// and `cd /repo && git ...` all get the three-variant treatment.
func bashVariants(value string) []string {
	first, rest := splitFirstToken(value)
	out := []string{
		"Bash(" + value + ")",
	}
	if first == "git" {
		out = append(out, "Bash(git -C /* "+rest+")")
	}
	out = append(out, "Bash(rtk "+value+")")
	if first == "git" {
		out = append(out, "Bash(rtk git -C /* "+rest+")")
	}
	out = append(out, "Bash(rtk proxy "+value+")")
	if first == "git" {
		out = append(out, "Bash(rtk proxy git -C /* "+rest+")")
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
