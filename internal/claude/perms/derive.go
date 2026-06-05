package perms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// unwrapBash returns the inner command of a `Bash(...)` rule and true,
// or ("", false) for any non-Bash rule (Read/WebFetch/Skill).
func unwrapBash(rule string) (string, bool) {
	const pre = "Bash("
	if strings.HasPrefix(rule, pre) && strings.HasSuffix(rule, ")") {
		return rule[len(pre) : len(rule)-1], true
	}
	return "", false
}

// toPrefix turns a command (or excludedCommands pattern) into its
// token-tuple: strip a trailing wildcard (" *", ":*", or "*"), peel
// leading FOO=bar env assignments, split on whitespace, and basename
// the leading token (so path twins collapse onto the bare command).
// Returns nil when nothing is left.
func toPrefix(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	switch {
	case strings.HasSuffix(cmd, " *"):
		cmd = cmd[:len(cmd)-2]
	case strings.HasSuffix(cmd, ":*"):
		cmd = cmd[:len(cmd)-2]
	case strings.HasSuffix(cmd, "*"):
		cmd = cmd[:len(cmd)-1]
	}
	fields := strings.FieldsFunc(cmd, unicode.IsSpace)
	i := 0
	for i < len(fields) && isAssignment(fields[i]) {
		i++
	}
	fields = fields[i:]
	if len(fields) == 0 {
		return nil
	}
	fields[0] = filepath.Base(fields[0])
	return fields
}

// isAssignment reports whether tok is a NAME=value env assignment
// (NAME is a non-empty [A-Za-z_][A-Za-z0-9_]* identifier).
func isAssignment(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for j, r := range tok[:eq] {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case j > 0 && unicode.IsDigit(r):
		default:
			return false
		}
	}
	return true
}

// Derive projects permissions.allow ∩ sandbox.excludedCommands into the
// two prefix lists the bash-allow-trusted hook consumes:
//   - excludedPrefixes: every excludedCommands pattern as a tuple.
//   - trusted: every Bash() allow rule whose tuple is prefixed by some
//     excluded tuple (i.e. the command is excluded). That prefix test
//     IS the "allowed ∩ excluded" relation.
//
// Both lists are sorted and deduped.
func Derive(allow, excluded []string) (trusted, excludedPrefixes [][]string) {
	for _, c := range excluded {
		if p := toPrefix(c); p != nil {
			excludedPrefixes = append(excludedPrefixes, p)
		}
	}
	excludedPrefixes = dedupTuples(excludedPrefixes)

	for _, rule := range allow {
		inner, ok := unwrapBash(rule)
		if !ok {
			continue
		}
		t := toPrefix(inner)
		if t != nil && anyPrefixOf(excludedPrefixes, t) {
			trusted = append(trusted, t)
		}
	}
	return dedupTuples(trusted), excludedPrefixes
}

// anyPrefixOf reports whether some tuple in prefixes is a (token-wise)
// prefix of t.
func anyPrefixOf(prefixes [][]string, t []string) bool {
	for _, e := range prefixes {
		if len(e) <= len(t) && equalTokens(e, t[:len(e)]) {
			return true
		}
	}
	return false
}

func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bashAllowInlineBudget is the maximum length (in characters, including
// the 2-space indent, key, colon, and any trailing comma) for which a
// top-level array field stays on a single line. Past it, each inner
// tuple gets its own line so long lists stay readable.
const bashAllowInlineBudget = 100

// WriteBashAllow writes {excluded, trusted} to path. Inner tuples are
// always inlined as ["a", "b"]; the outer array is inlined too when the
// whole field fits within bashAllowInlineBudget, otherwise it breaks to
// one tuple per line. Trailing newline. nil lists render as `[]`.
func WriteBashAllow(path string, trusted, excluded [][]string) error {
	if excluded == nil {
		excluded = [][]string{}
	}
	if trusted == nil {
		trusted = [][]string{}
	}
	var b strings.Builder
	b.WriteString("{\n")
	writeBashAllowField(&b, "excluded", excluded, true)
	writeBashAllowField(&b, "trusted", trusted, false)
	b.WriteString("}\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil { //nolint:gosec // conventional config-file perms
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// writeBashAllowField writes one top-level array field of bash-allow.
// trailingComma is true when another field still follows in the object.
func writeBashAllowField(b *strings.Builder, name string, tuples [][]string, trailingComma bool) {
	tail := ""
	if trailingComma {
		tail = ","
	}
	if len(tuples) == 0 {
		fmt.Fprintf(b, "  %q: []%s\n", name, tail)
		return
	}
	inner := make([]string, len(tuples))
	for i, t := range tuples {
		inner[i] = inlineTuple(t)
	}
	oneLine := fmt.Sprintf("  %q: [%s]%s", name, strings.Join(inner, ", "), tail)
	if len(oneLine) <= bashAllowInlineBudget {
		b.WriteString(oneLine)
		b.WriteByte('\n')
		return
	}
	fmt.Fprintf(b, "  %q: [\n", name)
	for i, t := range inner {
		sep := ","
		if i == len(inner)-1 {
			sep = ""
		}
		fmt.Fprintf(b, "    %s%s\n", t, sep)
	}
	fmt.Fprintf(b, "  ]%s\n", tail)
}

// inlineTuple renders a string tuple as `["a", "b"]` — JSON-quoted
// elements with a space after each comma.
func inlineTuple(tuple []string) string {
	parts := make([]string, len(tuple))
	for i, s := range tuple {
		q, err := json.Marshal(s)
		if err != nil {
			// json.Marshal on a string is total — this branch is
			// unreachable, but the linter wants the error checked.
			panic(fmt.Errorf("marshal tuple element %q: %w", s, err))
		}
		parts[i] = string(q)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// dedupTuples returns a copy with duplicate tuples removed and the
// result sorted by NUL-joined key for stable JSON output.
func dedupTuples(in [][]string) [][]string {
	seen := map[string]struct{}{}
	out := make([][]string, 0, len(in))
	for _, t := range in {
		k := strings.Join(t, "\x00")
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.Join(out[i], "\x00") < strings.Join(out[j], "\x00")
	})
	return out
}
