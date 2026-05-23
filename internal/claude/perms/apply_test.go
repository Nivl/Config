package perms

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// freshSettings returns a Settings backed by a fresh tempdir path so
// every test gets an isolated empty starting state.
func freshSettings(t *testing.T) *Settings {
	t.Helper()
	s, err := Load(filepath.Join(t.TempDir(), "settings.json"))
	require.NoError(t, err)
	return s
}

// TestAdd_BashGitExpandsToTwoVariants — a single git op produces the
// 2 expected rule strings (raw + cwd-bypass), both flowing through to
// the target list.
func TestAdd_BashGitExpandsToTwoVariants(t *testing.T) {
	s := freshSettings(t)
	diff, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "git status *"}}, false)
	require.NoError(t, err)
	assert.Len(t, diff.Added, 2)
	assert.Empty(t, diff.Skipped)
	assert.Contains(t, s.List(ListAllow), "Bash(git status *)")
	assert.Contains(t, s.List(ListAllow), "Bash(git -C /* status *)")
}

// TestAdd_AlreadyPresentIsSkipped — adding a rule that's already in
// the target list reports it as Skipped without touching anything.
func TestAdd_AlreadyPresentIsSkipped(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAllow, []string{"Bash(ls)"})

	diff, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}}, false)
	require.NoError(t, err)
	assert.Empty(t, diff.Added)
	assert.Len(t, diff.Skipped, 1, "the single variant already exists; it should be skipped")
}

// TestAdd_PartiallyPresentDedupesPerVariant — when one of a git
// command's two variants is already present, Add adds just the
// missing one and skips the present one.
func TestAdd_PartiallyPresentDedupesPerVariant(t *testing.T) {
	s := freshSettings(t)
	// Pre-seed only the raw variant; the cwd-bypass form is missing.
	s.SetList(ListAllow, []string{"Bash(git status *)"})

	diff, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "git status *"}}, false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Bash(git -C /* status *)"}, diff.Added)
	assert.ElementsMatch(t, []string{"Bash(git status *)"}, diff.Skipped)
}

// TestAdd_CrossListConflictErrorsWithoutForce — a rule that already
// lives in `ask` while the caller is adding to `allow` triggers
// ConflictError; the Settings is left untouched so the caller can
// retry with force=true.
func TestAdd_CrossListConflictErrorsWithoutForce(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)"})

	_, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}}, false)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, "Bash(ls)", conflictErr.Conflicts[0].Rule)
	assert.Equal(t, ListAsk, conflictErr.Conflicts[0].In)

	// Settings must not have been mutated.
	assert.Equal(t, []string{"Bash(ls)"}, s.List(ListAsk))
	assert.Empty(t, s.List(ListAllow))
}

// TestAdd_CrossListConflictFlattensMultipleListsAndOps — a single
// ConflictError collects every conflicting rule across every other
// list so the user gets the full picture in one report.
func TestAdd_CrossListConflictFlattensMultipleListsAndOps(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)"})
	s.SetList(ListDeny, []string{"Read(/etc/passwd)"})

	_, err := Add(s, ListAllow, []Op{
		{Kind: KindBash, Value: "ls"},
		{Kind: KindRead, Value: "/etc/passwd"},
	}, false)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 2)
}

// TestAdd_MultiListPresenceReportsAllSourcesAsConflicts — when a
// rule (illegally, via hand-edit) lives in two non-target lists,
// findConflicts must report both rather than break on the first
// match. Otherwise the user wouldn't know they need --force to
// resolve every source.
func TestAdd_MultiListPresenceReportsAllSourcesAsConflicts(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)"})
	s.SetList(ListDeny, []string{"Bash(ls)"})

	_, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}}, false)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	// Expect both the ask and deny entries reported.
	var lists []ListName
	for _, c := range conflictErr.Conflicts {
		if c.Rule == "Bash(ls)" {
			lists = append(lists, c.In)
		}
	}
	assert.ElementsMatch(t, []ListName{ListAsk, ListDeny}, lists,
		"a rule in two non-target lists must produce two conflicts, not just one")
}

// TestAdd_ForceMovesAllSourcesNotJustOne — same multi-list scenario
// but with --force=true: the rule must be removed from BOTH ask and
// deny, not just the first one found.
func TestAdd_ForceMovesAllSourcesNotJustOne(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)"})
	s.SetList(ListDeny, []string{"Bash(ls)"})

	diff, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}}, true)
	require.NoError(t, err)
	assert.Len(t, diff.Moved, 2, "both source lists should produce a Moved entry")
	assert.Contains(t, s.List(ListAllow), "Bash(ls)")
	assert.NotContains(t, s.List(ListAsk), "Bash(ls)", "force must clear all sources, not stop at first")
	assert.NotContains(t, s.List(ListDeny), "Bash(ls)", "force must clear all sources, not stop at first")
}

// TestAdd_ForceMovesRuleFromOtherList — force=true tells Add to
// re-categorize: the rule is removed from its old list and added to
// target, with the move recorded in Diff.Moved.
func TestAdd_ForceMovesRuleFromOtherList(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)", "Bash(other)"})

	diff, err := Add(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}}, true)
	require.NoError(t, err)
	require.Len(t, diff.Moved, 1)
	assert.Equal(t, "Bash(ls)", diff.Moved[0].Rule)
	assert.Equal(t, ListAsk, diff.Moved[0].From)
	assert.NotContains(t, diff.Added, "Bash(ls)", "moved rules must not also appear in Added")

	assert.Contains(t, s.List(ListAllow), "Bash(ls)")
	assert.NotContains(t, s.List(ListAsk), "Bash(ls)")
	assert.Contains(t, s.List(ListAsk), "Bash(other)", "unrelated rules in source list are unchanged")
}

// TestRemove_DropsExistingRules — Remove deletes target-list entries
// that match the expanded variants and reports them in Removed. Using
// a git command exercises the 2-variant expansion.
func TestRemove_DropsExistingRules(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAllow, []string{
		"Bash(git status *)", "Bash(git -C /* status *)", "Bash(keep)",
	})

	diff, err := Remove(s, ListAllow, []Op{{Kind: KindBash, Value: "git status *"}})
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{"Bash(git status *)", "Bash(git -C /* status *)"},
		diff.Removed)
	assert.Equal(t, []string{"Bash(keep)"}, s.List(ListAllow))
}

// TestRemove_NotPresentIsSkipped — Remove of a rule that isn't in
// target reports Skipped (no error) and never touches other lists,
// even if the rule happens to live there.
func TestRemove_NotPresentIsSkipped(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls)"})

	diff, err := Remove(s, ListAllow, []Op{{Kind: KindBash, Value: "ls"}})
	require.NoError(t, err)
	assert.Empty(t, diff.Removed)
	assert.Len(t, diff.Skipped, 1)
	assert.Equal(t, []string{"Bash(ls)"}, s.List(ListAsk),
		"Remove must never affect non-target lists")
}

// TestAdd_DedupesIntraBatchVariants — same op specified twice in one
// invocation expands to overlapping variants; the inner dedup keeps
// the per-rule reporting honest (each rule appears once in the Diff).
func TestAdd_DedupesIntraBatchVariants(t *testing.T) {
	s := freshSettings(t)
	diff, err := Add(s, ListAllow, []Op{
		{Kind: KindBash, Value: "ls"},
		{Kind: KindBash, Value: "ls"},
	}, false)
	require.NoError(t, err)
	assert.Len(t, diff.Added, 1, "duplicate op must not re-add rules")
}

// TestDiff_EmptyReflectsNoMutations — Empty is true only when no
// rules were Added, Removed, or Moved (Skipped doesn't count, since
// it represents no-op decisions, not on-disk changes).
func TestDiff_EmptyReflectsNoMutations(t *testing.T) {
	assert.True(t, Diff{}.Empty())
	assert.True(t, Diff{Skipped: []string{"Bash(x)"}}.Empty(),
		"skipped-only diffs are no-op writes")
	assert.False(t, Diff{Added: []string{"Bash(x)"}}.Empty())
	assert.False(t, Diff{Removed: []string{"Bash(x)"}}.Empty())
	assert.False(t, Diff{Moved: []MovedRule{{Rule: "x", From: ListAsk}}}.Empty())
}

// TestExpandOps_EmptyValueErrors — an Op carrying an empty value
// surfaces as an error from expandOps (via Variants), not a silent
// drop. Defensive: the cmd layer should already reject empties.
func TestExpandOps_EmptyValueErrors(t *testing.T) {
	_, err := expandOps([]Op{{Kind: KindBash, Value: ""}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty value")
}

// TestConflictError_ErrorString — the public Error() method on
// *ConflictError is what callers see when they Println the wrapped
// error. Lock in the exact shape so the cmd layer's user-facing
// formatting (which re-renders these for line-by-line display) has
// a stable contract to depend on.
func TestConflictError_ErrorString(t *testing.T) {
	t.Run("single conflict", func(t *testing.T) {
		err := &ConflictError{Conflicts: []Conflict{{Rule: "Bash(ls)", In: ListAsk}}}
		assert.Equal(t, "cross-list conflicts (re-run with --force to move): Bash(ls) in ask", err.Error())
	})
	t.Run("multiple conflicts joined by comma-space", func(t *testing.T) {
		err := &ConflictError{Conflicts: []Conflict{
			{Rule: "Bash(ls)", In: ListAsk},
			{Rule: "Read(secrets.txt)", In: ListDeny},
		}}
		assert.Equal(t,
			"cross-list conflicts (re-run with --force to move): Bash(ls) in ask, Read(secrets.txt) in deny",
			err.Error())
	})
	t.Run("zero conflicts still produces the prefix", func(t *testing.T) {
		// Defensive: callers shouldn't construct a ConflictError with
		// no conflicts, but if they do, the message stays grammatical.
		err := &ConflictError{Conflicts: nil}
		assert.Equal(t, "cross-list conflicts (re-run with --force to move): ", err.Error())
	})
}
