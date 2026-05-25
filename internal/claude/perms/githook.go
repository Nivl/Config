package perms

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// GitHookList identifies one of the three prefix tables in the Python
// git-safe-subcommands.py hook. The string value is the literal
// Python identifier used in the file, so callers can pass it
// straight into log/error text.
type GitHookList string

const (
	// GitHookAllow names the ALLOW_PREFIXES tuple — prefixes here
	// produce permissionDecision="allow" from the hook.
	GitHookAllow GitHookList = "ALLOW_PREFIXES"
	// GitHookAsk names the ASK_PREFIXES tuple — prefixes here force
	// a permission prompt even if the command would otherwise
	// auto-allow.
	GitHookAsk GitHookList = "ASK_PREFIXES"
	// GitHookDeny names the DENY_PREFIXES tuple — prefixes here
	// block the command unconditionally.
	GitHookDeny GitHookList = "DENY_PREFIXES"
)

// AllGitHookLists returns the three hook lists in evaluation order
// (allow, ask, deny). Used by conflict detection and by Save to keep
// the rendered file deterministic.
func AllGitHookLists() []GitHookList {
	return []GitHookList{GitHookAllow, GitHookAsk, GitHookDeny}
}

// GitHookFile is an in-memory mirror of the git-safe-subcommands.py
// hook. It keeps the full file text so unrelated content (imports,
// helpers, the main function) round-trips byte-for-byte, and exposes
// only the three prefix tables as typed slice-of-slices for the
// cmd layer to mutate.
//
// Each prefix is `[]string{"subcommand", "subarg", ...}` — the
// token tuple the Python hook matches against the post-`git` args.
type GitHookFile struct {
	lists map[GitHookList][][]string
	body  string
}

// LoadGitHook reads path and parses the three prefix tables.
// Returns an error if any table is missing or malformed — the cmd
// layer should treat that as a corrupt hook file rather than
// silently dropping entries.
func LoadGitHook(path string) (*GitHookFile, error) {
	data, err := os.ReadFile(path) //nolint:gosec // project-tracked hook file
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	h := &GitHookFile{
		body:  string(data),
		lists: map[GitHookList][][]string{},
	}
	for _, name := range AllGitHookLists() {
		prefixes, err := parseGitHookBlock(h.body, string(name))
		if err != nil {
			return nil, fmt.Errorf("parse %s in %s: %w", name, path, err)
		}
		h.lists[name] = prefixes
	}
	return h, nil
}

// List returns the prefixes currently in name. The returned slice is
// owned by GitHookFile — callers should treat it as read-only and
// route mutations through Add/Remove.
func (h *GitHookFile) List(name GitHookList) [][]string {
	return h.lists[name]
}

// Has reports whether prefix is currently in name.
func (h *GitHookFile) Has(name GitHookList, prefix []string) bool {
	for _, p := range h.lists[name] {
		if slices.Equal(p, prefix) {
			return true
		}
	}
	return false
}

// Add inserts prefix into name and returns true. If prefix is
// already present, returns false without mutating.
func (h *GitHookFile) Add(name GitHookList, prefix []string) bool {
	if h.Has(name, prefix) {
		return false
	}
	cloned := append([]string(nil), prefix...)
	h.lists[name] = append(h.lists[name], cloned)
	return true
}

// Remove drops the first occurrence of prefix from name and returns
// true. Returns false if prefix wasn't there.
func (h *GitHookFile) Remove(name GitHookList, prefix []string) bool {
	list := h.lists[name]
	for i, p := range list {
		if slices.Equal(p, prefix) {
			h.lists[name] = append(list[:i:i], list[i+1:]...)
			return true
		}
	}
	return false
}

// Save renders the three prefix tables back into the file body —
// sorted alphabetically by token, deduped, with stable Python
// formatting — and writes the result to path. The rest of the file
// (imports, helpers, the main function) is preserved verbatim.
func (h *GitHookFile) Save(path string) error {
	body := h.body
	for _, name := range AllGitHookLists() {
		rendered := renderGitHookBlock(string(name), h.lists[name])
		next, err := replaceGitHookBlock(body, string(name), rendered)
		if err != nil {
			return fmt.Errorf("replace %s: %w", name, err)
		}
		body = next
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { //nolint:gosec // conventional config-file perms
		return fmt.Errorf("write %s: %w", path, err)
	}
	h.body = body
	return nil
}

// parseGitHookBlock locates `<name> = (...)` in content and returns
// the parsed prefixes. Recognises both forms the writer emits:
//
//	NAME = ()                       # empty
//	NAME = (\n    ("a",),\n)        # non-empty
func parseGitHookBlock(content, name string) ([][]string, error) {
	emptyRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\(\s*\)\s*$`)
	if emptyRe.MatchString(content) {
		return [][]string{}, nil
	}

	openRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\(\s*$`)
	openLoc := openRe.FindStringIndex(content)
	if openLoc == nil {
		return nil, errors.New("block not found")
	}
	rest := content[openLoc[1]:]
	closeRe := regexp.MustCompile(`(?m)^\)\s*$`)
	closeLoc := closeRe.FindStringIndex(rest)
	if closeLoc == nil {
		return nil, errors.New("closing `)` not found")
	}

	var prefixes [][]string
	for line := range strings.SplitSeq(rest[:closeLoc[0]], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		prefix, err := parseGitHookTupleLine(line)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

// replaceGitHookBlock substitutes the existing `<name> = (...)`
// block in content with rendered, preserving everything else.
// rendered must not include a trailing newline — the surrounding
// newline structure is taken from the original block boundaries.
func replaceGitHookBlock(content, name, rendered string) (string, error) {
	emptyRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\(\s*\)\s*$`)
	if loc := emptyRe.FindStringIndex(content); loc != nil {
		return content[:loc[0]] + rendered + content[loc[1]:], nil
	}
	openRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*=\s*\(\s*$`)
	openLoc := openRe.FindStringIndex(content)
	if openLoc == nil {
		return "", errors.New("block not found")
	}
	rest := content[openLoc[1]:]
	closeRe := regexp.MustCompile(`(?m)^\)\s*$`)
	closeLoc := closeRe.FindStringIndex(rest)
	if closeLoc == nil {
		return "", errors.New("closing `)` not found")
	}
	end := openLoc[1] + closeLoc[1]
	return content[:openLoc[0]] + rendered + content[end:], nil
}

// tupleLineRe captures the `("a", "b", ...)` payload from one line
// of a prefix tuple. The trailing comma between tuples is consumed
// outside the capture group.
var tupleLineRe = regexp.MustCompile(`^\((.*)\)\s*,?\s*$`)

// stringTokenRe captures one Python double-quoted string literal,
// including support for escaped quotes (`\"`).
var stringTokenRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// parseGitHookTupleLine turns one rendered line — e.g.
// `    ("merge", "origin/main"),` — back into ["merge","origin/main"].
// Reports a clear error for malformed input so corrupt hook files
// fail loudly rather than silently dropping entries.
func parseGitHookTupleLine(line string) ([]string, error) {
	m := tupleLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil, fmt.Errorf("malformed tuple line: %q", line)
	}
	var tokens []string
	for _, sm := range stringTokenRe.FindAllStringSubmatch(m[1], -1) {
		tokens = append(tokens, unescapePyString(sm[1]))
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty tuple: %q", line)
	}
	return tokens, nil
}

// unescapePyString reverses the limited escaping that renderTuple
// emits: backslash-escaped backslashes and double quotes. Anything
// else passes through (we never write other escapes, so seeing one
// means the file was hand-edited and the user can deal with it).
func unescapePyString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// renderGitHookBlock formats name + its prefixes as Python source
// text matching the hand-authored convention in the seeded file:
// empty lists collapse to `NAME = ()`, non-empty lists span multiple
// lines with one tuple per line indented four spaces. Prefixes are
// sorted lexicographically by token for stable diffs.
func renderGitHookBlock(name string, prefixes [][]string) string {
	sorted := append([][]string(nil), prefixes...)
	slices.SortFunc(sorted, slices.Compare[[]string])
	deduped := dedupePrefixes(sorted)

	if len(deduped) == 0 {
		return name + " = ()"
	}
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(" = (\n")
	for _, prefix := range deduped {
		b.WriteString("    ")
		b.WriteString(renderGitHookTuple(prefix))
		b.WriteString(",\n")
	}
	b.WriteString(")")
	return b.String()
}

// dedupePrefixes drops consecutive duplicates. Input must already
// be sorted (slices.SortFunc applies the same compare we use here),
// so any duplicate pair is adjacent.
func dedupePrefixes(prefixes [][]string) [][]string {
	if len(prefixes) == 0 {
		return prefixes
	}
	out := prefixes[:1]
	for _, p := range prefixes[1:] {
		if slices.Equal(p, out[len(out)-1]) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// renderGitHookTuple emits a single tuple in Python literal form.
// 1-element tuples need the trailing comma (`("a",)`) or Python
// reads them as a parenthesized expression; 2+ element tuples
// don't, and we omit it to match the seeded file's style.
func renderGitHookTuple(prefix []string) string {
	quoted := make([]string, len(prefix))
	for i, t := range prefix {
		esc := strings.ReplaceAll(t, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		quoted[i] = `"` + esc + `"`
	}
	if len(quoted) == 1 {
		return "(" + quoted[0] + ",)"
	}
	return "(" + strings.Join(quoted, ", ") + ")"
}

// listNameToGitHookList maps the user-facing list name (allow/ask/
// deny) to the corresponding Python identifier. Used at the API
// boundary so callers stay in ListName-land.
func listNameToGitHookList(n ListName) (GitHookList, error) {
	switch n {
	case ListAllow:
		return GitHookAllow, nil
	case ListAsk:
		return GitHookAsk, nil
	case ListDeny:
		return GitHookDeny, nil
	}
	return "", fmt.Errorf("unknown list name %q", n)
}

// gitHookListToListName is the inverse mapping. Used when surfacing
// hook-side moves in the Diff (which uses ListName).
func gitHookListToListName(g GitHookList) ListName {
	switch g {
	case GitHookAllow:
		return ListAllow
	case GitHookAsk:
		return ListAsk
	case GitHookDeny:
		return ListDeny
	}
	return ""
}

// renderHookRule formats a hook prefix as the user-facing rule
// string it would have lived under in settings.json — e.g.
// ["show"] → "Bash(git show *)". The Diff carries these strings
// to printDiff so users see consistent rule shapes regardless of
// which file the rule physically lives in.
func renderHookRule(prefix []string) string {
	return "Bash(git " + strings.Join(prefix, " ") + " *)"
}

// AddGitHook adds prefixes to target inside h. Mirrors Add for the
// hook backend: already-present → Skipped; in another list with
// force=false → ConflictError; in another list with force=true →
// Moved; otherwise → Added.
//
// ConflictError leaves h untouched so callers can retry with force.
func AddGitHook(h *GitHookFile, target ListName, prefixes [][]string, force bool) (Diff, error) {
	targetHook, err := listNameToGitHookList(target)
	if err != nil {
		return Diff{}, err
	}
	if !force {
		if conflicts := findGitHookConflicts(h, targetHook, prefixes); len(conflicts) > 0 {
			return Diff{}, &ConflictError{Conflicts: conflicts}
		}
	}
	diff := Diff{}
	for _, prefix := range prefixes {
		ruleStr := renderHookRule(prefix)
		if h.Has(targetHook, prefix) {
			diff.HookSkipped = append(diff.HookSkipped, ruleStr)
			continue
		}
		moved := false
		for _, other := range AllGitHookLists() {
			if other == targetHook {
				continue
			}
			if !h.Has(other, prefix) {
				continue
			}
			h.Remove(other, prefix)
			diff.HookMoved = append(diff.HookMoved, MovedRule{Rule: ruleStr, From: gitHookListToListName(other)})
			moved = true
		}
		h.Add(targetHook, prefix)
		if !moved {
			diff.HookAdded = append(diff.HookAdded, ruleStr)
		}
	}
	return diff, nil
}

// RemoveGitHook drops prefixes from target inside h. Mirrors Remove
// for the hook backend: prefixes not in target (whether absent or
// in another list) are Skipped — Remove never touches other lists.
func RemoveGitHook(h *GitHookFile, target ListName, prefixes [][]string) (Diff, error) {
	targetHook, err := listNameToGitHookList(target)
	if err != nil {
		return Diff{}, err
	}
	diff := Diff{}
	for _, prefix := range prefixes {
		ruleStr := renderHookRule(prefix)
		if !h.Has(targetHook, prefix) {
			diff.HookSkipped = append(diff.HookSkipped, ruleStr)
			continue
		}
		h.Remove(targetHook, prefix)
		diff.HookRemoved = append(diff.HookRemoved, ruleStr)
	}
	return diff, nil
}

// findGitHookConflicts mirrors findConflicts for the hook backend.
// Walks every non-target list and records a Conflict for each
// prefix that already lives there. Sorted to match the rendering
// of settings-side conflicts.
func findGitHookConflicts(h *GitHookFile, target GitHookList, prefixes [][]string) []Conflict {
	out := make([]Conflict, 0, len(prefixes))
	for _, prefix := range prefixes {
		for _, name := range AllGitHookLists() {
			if name == target {
				continue
			}
			if h.Has(name, prefix) {
				out = append(out, Conflict{Rule: renderHookRule(prefix), In: gitHookListToListName(name)})
			}
		}
	}
	return out
}

// SplitOpsByBackend separates a batched op slice into the two
// backends: settingsOps stay in the existing settings-store flow,
// hookPrefixes (parsed from git+Bash ops) feed the hook backend.
// Returns the first parse error encountered so the caller can
// surface it before any mutation begins.
func SplitOpsByBackend(ops []Op) (settingsOps []Op, hookPrefixes [][]string, err error) {
	for _, op := range ops {
		if op.Kind == KindBash {
			prefix, isGit, perr := ParseGitPrefix(op.Value)
			if perr != nil {
				return nil, nil, fmt.Errorf("op %q: %w", op.Value, perr)
			}
			if isGit {
				hookPrefixes = append(hookPrefixes, prefix)
				continue
			}
		}
		settingsOps = append(settingsOps, op)
	}
	return settingsOps, hookPrefixes, nil
}

// ApplyAction selects the verb passed to Apply.
type ApplyAction int

const (
	// ApplyAdd routes ops through Add (settings) and AddGitHook (hook).
	ApplyAdd ApplyAction = iota
	// ApplyRemove routes ops through Remove (settings) and RemoveGitHook.
	ApplyRemove
)

// Apply is the high-level entrypoint for `claude perms <list>
// add/remove`. It splits ops by backend, pre-flights cross-list
// conflicts across BOTH backends (so a mixed-batch failure leaves
// neither file mutated), then runs the underlying Add/Remove on
// each side and merges their Diffs.
//
// On ConflictError, neither s nor h is mutated. On any non-conflict
// error, mutation state is undefined — callers should treat the
// error as fatal and abandon both objects.
func Apply(s *Settings, h *GitHookFile, target ListName, ops []Op, action ApplyAction, force bool) (Diff, error) {
	settingsOps, hookPrefixes, err := SplitOpsByBackend(ops)
	if err != nil {
		return Diff{}, err
	}
	if action == ApplyAdd && !force {
		settingsRules, err := expandOps(settingsOps)
		if err != nil {
			return Diff{}, err
		}
		targetHook, _ := listNameToGitHookList(target) // safe: ListName values are exhaustive
		settingsConflicts := findConflicts(s, target, settingsRules)
		hookConflicts := findGitHookConflicts(h, targetHook, hookPrefixes)
		conflicts := make([]Conflict, 0, len(settingsConflicts)+len(hookConflicts))
		conflicts = append(conflicts, settingsConflicts...)
		conflicts = append(conflicts, hookConflicts...)
		if len(conflicts) > 0 {
			sortConflicts(conflicts)
			return Diff{}, &ConflictError{Conflicts: conflicts}
		}
	}

	var settingsDiff, hookDiff Diff
	switch action {
	case ApplyAdd:
		settingsDiff, err = Add(s, target, settingsOps, force)
		if err != nil {
			return Diff{}, err
		}
		hookDiff, err = AddGitHook(h, target, hookPrefixes, force)
		if err != nil {
			return Diff{}, err
		}
	case ApplyRemove:
		settingsDiff, err = Remove(s, target, settingsOps)
		if err != nil {
			return Diff{}, err
		}
		hookDiff, err = RemoveGitHook(h, target, hookPrefixes)
		if err != nil {
			return Diff{}, err
		}
	default:
		return Diff{}, fmt.Errorf("unknown ApplyAction %d", action)
	}
	return mergeDiffs(settingsDiff, hookDiff), nil
}

// mergeDiffs concatenates two diffs field-by-field. The settings
// half always owns Added/Removed/Skipped/Moved; the hook half
// always owns the Hook* counterparts; no field is populated by
// both halves so this is a pure merge.
func mergeDiffs(settings, hook Diff) Diff {
	return Diff{
		Added:       settings.Added,
		Removed:     settings.Removed,
		Skipped:     settings.Skipped,
		Moved:       settings.Moved,
		HookAdded:   hook.HookAdded,
		HookRemoved: hook.HookRemoved,
		HookSkipped: hook.HookSkipped,
		HookMoved:   hook.HookMoved,
	}
}

// sortConflicts orders conflicts by (rule, list) so the rendered
// error message is deterministic across runs and across the two
// backend sources.
func sortConflicts(c []Conflict) {
	slices.SortFunc(c, func(a, b Conflict) int {
		if a.Rule != b.Rule {
			if a.Rule < b.Rule {
				return -1
			}
			return 1
		}
		if a.In < b.In {
			return -1
		}
		if a.In > b.In {
			return 1
		}
		return 0
	})
}
