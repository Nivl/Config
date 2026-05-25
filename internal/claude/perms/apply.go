package perms

import (
	"fmt"
	"sort"
	"strings"
)

// Op is one user-supplied operand: a (Kind, Value) pair that
// Variants() will expand into one or more rule strings.
type Op struct {
	Value string
	Kind  Kind
}

// Diff records what an Add or Remove invocation actually changed.
// The cmd layer prints these to stderr so the user sees every rule
// that was added, removed, skipped (already in the right state), or
// moved (re-categorized via --force).
//
// Settings-store changes (Added/Removed/Skipped/Moved) and git-hook
// changes (HookAdded/HookRemoved/HookSkipped/HookMoved) are
// reported separately so the cmd layer knows which file(s) to
// stage. Hook entries are rendered as human-readable rule strings
// (e.g. "Bash(git show *)") for diff output, even though they're
// stored on disk as Python prefix tuples.
type Diff struct {
	// Added lists rules that were new to the target list.
	Added []string
	// Removed lists rules that were taken out of the target list.
	Removed []string
	// Skipped lists rules that were already in the desired state
	// (already-present for Add, already-absent for Remove). Reported
	// so users can audit batched invocations.
	Skipped []string
	// Moved lists rules that were re-categorized via --force
	// (removed from one list and added to the target list).
	Moved []MovedRule

	// HookAdded lists git hook prefixes that were new to the target
	// list, rendered as readable rule strings.
	HookAdded []string
	// HookRemoved lists git hook prefixes that were taken out of
	// the target list, rendered as readable rule strings.
	HookRemoved []string
	// HookSkipped lists git hook prefixes already in the desired
	// state, rendered as readable rule strings.
	HookSkipped []string
	// HookMoved lists git hook prefixes re-categorized via --force.
	HookMoved []MovedRule
}

// MovedRule describes one rule that --force re-categorized between
// lists during an Add. From identifies the list the rule was
// evicted from.
type MovedRule struct {
	Rule string
	From ListName
}

// Empty reports whether the Diff records no on-disk changes — Save
// is a no-op in that case, so the cmd layer can skip the git commit.
func (d Diff) Empty() bool {
	return !d.SettingsChanged() && !d.HookChanged()
}

// SettingsChanged reports whether the settings.json file would be
// modified by this diff. Independent of HookChanged so the cmd
// layer can stage one or both files as appropriate.
func (d Diff) SettingsChanged() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Moved) > 0
}

// HookChanged reports whether the git hook file would be modified
// by this diff. Independent of SettingsChanged.
func (d Diff) HookChanged() bool {
	return len(d.HookAdded) > 0 || len(d.HookRemoved) > 0 || len(d.HookMoved) > 0
}

// Add expands ops into rule strings and adds each to target.
//
// Three outcomes per rule:
//   - Already present in target          → Skipped.
//   - Present in a different list, force=false → returns ConflictError
//     after collecting all conflicts (no mutation).
//   - Present in a different list, force=true  → Moved (removed from
//     old list, added to target).
//   - Not present anywhere                → Added.
//
// On ConflictError the in-memory Settings is left unchanged; callers
// can re-run with force=true after surfacing the conflicts to the user.
func Add(s *Settings, target ListName, ops []Op, force bool) (Diff, error) {
	rules, err := expandOps(ops)
	if err != nil {
		return Diff{}, err
	}

	if !force {
		conflicts := findConflicts(s, target, rules)
		if len(conflicts) > 0 {
			return Diff{}, &ConflictError{Conflicts: conflicts}
		}
	}

	indexes := buildIndexes(s)
	diff := Diff{}
	for _, rule := range rules {
		if _, ok := indexes[target][rule]; ok {
			diff.Skipped = append(diff.Skipped, rule)
			continue
		}
		// A well-formed Settings holds each rule in at most one list,
		// but a hand-edit of settings.json could violate that. Walk
		// every other list (don't break on first match) so the move
		// is total and the Diff reports each source.
		moved := false
		for _, other := range AllLists() {
			if other == target {
				continue
			}
			if _, ok := indexes[other][rule]; !ok {
				continue
			}
			s.SetList(other, removeOne(s.List(other), rule))
			delete(indexes[other], rule)
			diff.Moved = append(diff.Moved, MovedRule{Rule: rule, From: other})
			moved = true
		}
		s.SetList(target, append(s.List(target), rule))
		indexes[target][rule] = struct{}{}
		if !moved {
			diff.Added = append(diff.Added, rule)
		}
	}
	return diff, nil
}

// Remove expands ops into rule strings and drops each from target.
// Rules that aren't currently in target (whether absent entirely or
// living in a different list) are Skipped — Remove never touches
// other lists, only target.
func Remove(s *Settings, target ListName, ops []Op) (Diff, error) {
	rules, err := expandOps(ops)
	if err != nil {
		return Diff{}, err
	}

	indexes := buildIndexes(s)
	diff := Diff{}
	for _, rule := range rules {
		if _, ok := indexes[target][rule]; !ok {
			diff.Skipped = append(diff.Skipped, rule)
			continue
		}
		s.SetList(target, removeOne(s.List(target), rule))
		delete(indexes[target], rule)
		diff.Removed = append(diff.Removed, rule)
	}
	return diff, nil
}

// ConflictError is returned by Add when one or more expanded rules
// already live in a different list than the target. Each Conflict
// names the rule and the list it currently occupies so the cmd
// layer can render an actionable error message.
type ConflictError struct {
	Conflicts []Conflict
}

// Conflict is one rule that lives in a different list than the
// caller is trying to add it to.
type Conflict struct {
	Rule string
	In   ListName
}

// Error renders all conflicts on a single line for cobra/printer
// consumption; the cmd layer typically formats them line-by-line.
func (e *ConflictError) Error() string {
	parts := make([]string, len(e.Conflicts))
	for i, c := range e.Conflicts {
		parts[i] = fmt.Sprintf("%s in %s", c.Rule, c.In)
	}
	return "cross-list conflicts (re-run with --force to move): " + strings.Join(parts, ", ")
}

// expandOps fans every Op out into its Variants and returns the
// concatenated, order-preserved rule list with intra-batch duplicates
// removed. We dedupe here so the per-rule reporting in Diff doesn't
// double-count a rule the user happened to specify twice.
func expandOps(ops []Op) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(ops)*3)
	for _, op := range ops {
		variants, err := Variants(op.Kind, op.Value)
		if err != nil {
			return nil, fmt.Errorf("op %v: %w", op, err)
		}
		for _, v := range variants {
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out, nil
}

// findConflicts returns every rule that is currently in a list
// *other than* target, so the caller (Add) can surface them together
// rather than failing on the first one. A rule that (illegally)
// lives in two non-target lists yields two Conflict entries so the
// user sees the full picture instead of a partial diagnosis.
func findConflicts(s *Settings, target ListName, rules []string) []Conflict {
	indexes := buildIndexes(s)
	var out []Conflict
	for _, rule := range rules {
		for _, name := range AllLists() {
			if name == target {
				continue
			}
			if _, ok := indexes[name][rule]; ok {
				out = append(out, Conflict{Rule: rule, In: name})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].In < out[j].In
	})
	return out
}

// buildIndexes returns a name → (rule → struct{}) lookup so the
// per-rule decisions in Add/Remove run in O(1) rather than O(N) per
// check. Rebuilt fresh on each call; cheap relative to file I/O.
func buildIndexes(s *Settings) map[ListName]map[string]struct{} {
	out := map[ListName]map[string]struct{}{}
	for _, name := range AllLists() {
		idx := map[string]struct{}{}
		for _, rule := range s.List(name) {
			idx[rule] = struct{}{}
		}
		out[name] = idx
	}
	return out
}

// removeOne returns a copy of list with the first occurrence of rule
// stripped. The list is small and we want a fresh slice (so callers
// don't hold onto a backing array we might grow) — clarity beats
// micro-optimization here.
func removeOne(list []string, rule string) []string {
	for i, r := range list {
		if r == rule {
			out := make([]string, 0, len(list)-1)
			out = append(out, list[:i]...)
			out = append(out, list[i+1:]...)
			return out
		}
	}
	return list
}
