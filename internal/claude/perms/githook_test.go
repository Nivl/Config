package perms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleHook returns a minimal git-safe-subcommands.py-shaped fixture
// covering all three forms the parser must round-trip: a non-empty
// allow list (single- and multi-token tuples), an empty ask list,
// and a deny list with one entry. The surrounding `# preamble` and
// `# trailer` lines anchor the preservation assertion: callers that
// mutate the lists must not perturb either.
func sampleHook() string {
	return `#!/usr/bin/env python3
# preamble
import os

ALLOW_PREFIXES = (
    ("add",),
    ("merge", "origin/main"),
    ("show",),
)

ASK_PREFIXES = ()

DENY_PREFIXES = (
    ("push", "--force"),
)
# trailer
`
}

// writeHookFile drops content into a tempdir under
// `git-safe-subcommands.py` and returns the path. Centralised so
// each test reads the same way as production code.
func writeHookFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "git-safe-subcommands.py")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// TestGitHook_Load_ParsesThreeBlocks — confirms the parser locates
// each of the three named blocks, populates the typed lists with
// the right number of tuples, and preserves intra-tuple token order.
func TestGitHook_Load_ParsesThreeBlocks(t *testing.T) {
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"add"},
		{"merge", "origin/main"},
		{"show"},
	}, h.List(GitHookAllow))
	assert.Empty(t, h.List(GitHookAsk))
	assert.Equal(t, [][]string{{"push", "--force"}}, h.List(GitHookDeny))
}

// TestGitHook_Load_FailsOnMissingBlock — a hook file with one
// missing block is an unrecoverable shape error, not a benign
// "empty list". The parser should bail so the cmd layer doesn't
// silently re-emit a truncated file.
func TestGitHook_Load_FailsOnMissingBlock(t *testing.T) {
	broken := `ALLOW_PREFIXES = ()
DENY_PREFIXES = ()
`
	_, err := LoadGitHook(writeHookFile(t, broken))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ASK_PREFIXES")
}

// TestGitHook_Load_FailsOnMalformedTuple — a syntactically broken
// tuple line (missing closing paren) must surface as an error, not
// silently produce an empty/partial list.
func TestGitHook_Load_FailsOnMalformedTuple(t *testing.T) {
	broken := `ALLOW_PREFIXES = (
    ("add",
)

ASK_PREFIXES = ()

DENY_PREFIXES = ()
`
	_, err := LoadGitHook(writeHookFile(t, broken))
	require.Error(t, err)
}

// TestGitHook_Add_NewPrefix — adding a never-seen prefix returns
// true and the new entry shows up in the list.
func TestGitHook_Add_NewPrefix(t *testing.T) {
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	added := h.Add(GitHookAllow, []string{"diff"})
	assert.True(t, added)
	assert.True(t, h.Has(GitHookAllow, []string{"diff"}))
}

// TestGitHook_Add_Duplicate — re-adding an existing prefix returns
// false (it's a no-op) so the cmd layer can mark it Skipped.
func TestGitHook_Add_Duplicate(t *testing.T) {
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	added := h.Add(GitHookAllow, []string{"add"})
	assert.False(t, added, "duplicate add should be a no-op")
}

// TestGitHook_Remove_Found — removing a present prefix returns
// true and the entry disappears from the list.
func TestGitHook_Remove_Found(t *testing.T) {
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	removed := h.Remove(GitHookAllow, []string{"show"})
	assert.True(t, removed)
	assert.False(t, h.Has(GitHookAllow, []string{"show"}))
}

// TestGitHook_Remove_NotFound — removing an absent prefix returns
// false; cmd layer renders that as Skipped.
func TestGitHook_Remove_NotFound(t *testing.T) {
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	removed := h.Remove(GitHookAllow, []string{"push"})
	assert.False(t, removed)
}

// TestGitHook_Save_RoundTrips — Save then Load must yield the same
// typed lists, proving the renderer is the inverse of the parser
// for the full sampleHook fixture.
func TestGitHook_Save_RoundTrips(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	require.NoError(t, h.Save(path))

	reloaded, err := LoadGitHook(path)
	require.NoError(t, err)
	assert.Equal(t, h.List(GitHookAllow), reloaded.List(GitHookAllow))
	assert.Equal(t, h.List(GitHookAsk), reloaded.List(GitHookAsk))
	assert.Equal(t, h.List(GitHookDeny), reloaded.List(GitHookDeny))
}

// TestGitHook_Save_PreservesPreambleAndTrailer — the line markers
// in sampleHook ("# preamble", "import os", "# trailer") must be
// preserved byte-for-byte on save. This is the load-bearing test
// against the regex-replace strategy clipping too much/too little.
func TestGitHook_Save_PreservesPreambleAndTrailer(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	require.True(t, h.Add(GitHookAsk, []string{"reset", "--hard"}))
	require.NoError(t, h.Save(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "#!/usr/bin/env python3")
	assert.Contains(t, body, "# preamble")
	assert.Contains(t, body, "import os")
	assert.Contains(t, body, "# trailer")
}

// TestGitHook_Save_EmptyToNonEmpty — when an empty `NAME = ()`
// list gains a prefix, the renderer must switch to the multi-line
// form. The single-line collapse is reserved for actually-empty
// tables.
func TestGitHook_Save_EmptyToNonEmpty(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	h.Add(GitHookAsk, []string{"reset", "--hard"})
	require.NoError(t, h.Save(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	body := string(data)
	assert.Contains(t, body, "ASK_PREFIXES = (\n")
	assert.Contains(t, body, "    (\"reset\", \"--hard\"),\n")
	assert.NotContains(t, body, "ASK_PREFIXES = ()")
}

// TestGitHook_Save_NonEmptyToEmpty — removing the last entry must
// collapse a list back to `NAME = ()` so the file stays compact.
func TestGitHook_Save_NonEmptyToEmpty(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	require.True(t, h.Remove(GitHookDeny, []string{"push", "--force"}))
	require.NoError(t, h.Save(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "DENY_PREFIXES = ()")
}

// TestGitHook_Save_SortsAndDedupes — the renderer sorts
// lexicographically by token tuple and drops duplicates so the
// on-disk file stays stable regardless of insertion order. The
// in-memory list is allowed to carry the duplicate (Add catches
// that earlier); the file shouldn't.
func TestGitHook_Save_SortsAndDedupes(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	// Insert in deliberate-wrong order. The renderer should pull
	// these into alphabetical order on disk.
	h.lists[GitHookAllow] = [][]string{
		{"zap"},
		{"add"},
		{"merge", "origin/main"},
		{"add"}, // duplicate
	}
	require.NoError(t, h.Save(path))

	reloaded, err := LoadGitHook(path)
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"add"},
		{"merge", "origin/main"},
		{"zap"},
	}, reloaded.List(GitHookAllow))
}

// TestGitHook_Save_EscapesSpecialChars — quotes and backslashes
// inside a token survive a render→parse cycle. Real prefixes
// shouldn't contain these, but a hand-edited file might, and
// silent corruption is worse than a noisy round-trip.
func TestGitHook_Save_EscapesSpecialChars(t *testing.T) {
	path := writeHookFile(t, sampleHook())
	h, err := LoadGitHook(path)
	require.NoError(t, err)
	weird := []string{`with"quote`, `with\back`}
	h.Add(GitHookAsk, weird)
	require.NoError(t, h.Save(path))

	reloaded, err := LoadGitHook(path)
	require.NoError(t, err)
	assert.True(t, reloaded.Has(GitHookAsk, weird))
}

// freshHook returns a LoadGitHook-backed *GitHookFile pointing at a
// fresh tempdir copy of sampleHook. Keeps Apply/AddGitHook tests
// from each tripping over the others' mutations.
func freshHook(t *testing.T) *GitHookFile {
	t.Helper()
	h, err := LoadGitHook(writeHookFile(t, sampleHook()))
	require.NoError(t, err)
	return h
}

// TestSplitOpsByBackend_SplitsCleanly — a mixed batch is split into
// settings ops and hook prefixes preserving input order within each
// backend.
func TestSplitOpsByBackend_SplitsCleanly(t *testing.T) {
	ops := []Op{
		{Kind: KindBash, Value: "ls *"},
		{Kind: KindBash, Value: "git show *"},
		{Kind: KindRead, Value: "/etc/foo"},
		{Kind: KindBash, Value: "git merge origin/main *"},
	}
	settingsOps, hookPrefixes, err := SplitOpsByBackend(ops)
	require.NoError(t, err)
	assert.Equal(t, []Op{
		{Kind: KindBash, Value: "ls *"},
		{Kind: KindRead, Value: "/etc/foo"},
	}, settingsOps)
	assert.Equal(t, [][]string{
		{"show"},
		{"merge", "origin/main"},
	}, hookPrefixes)
}

// TestSplitOpsByBackend_PropagatesParseError — a malformed git op
// fails the whole split so mutation never begins.
func TestSplitOpsByBackend_PropagatesParseError(t *testing.T) {
	_, _, err := SplitOpsByBackend([]Op{{Kind: KindBash, Value: "git status"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing `*`")
}

// TestAddGitHook_NewPrefix — adding a never-seen prefix records it
// under HookAdded with the readable rule string.
func TestAddGitHook_NewPrefix(t *testing.T) {
	h := freshHook(t)
	diff, err := AddGitHook(h, ListAllow, [][]string{{"diff"}}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(git diff *)"}, diff.HookAdded)
	assert.True(t, h.Has(GitHookAllow, []string{"diff"}))
}

// TestAddGitHook_Duplicate — re-adding records HookSkipped without
// mutating the file.
func TestAddGitHook_Duplicate(t *testing.T) {
	h := freshHook(t)
	diff, err := AddGitHook(h, ListAllow, [][]string{{"add"}}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(git add *)"}, diff.HookSkipped)
	assert.Empty(t, diff.HookAdded)
}

// TestAddGitHook_CrossListConflict — adding to allow a prefix that
// currently lives in deny yields ConflictError with no mutation.
func TestAddGitHook_CrossListConflict(t *testing.T) {
	h := freshHook(t)
	_, err := AddGitHook(h, ListAllow, [][]string{{"push", "--force"}}, false)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, "Bash(git push --force *)", conflictErr.Conflicts[0].Rule)
	assert.Equal(t, ListDeny, conflictErr.Conflicts[0].In)
	// Still in deny, not in allow.
	assert.True(t, h.Has(GitHookDeny, []string{"push", "--force"}))
	assert.False(t, h.Has(GitHookAllow, []string{"push", "--force"}))
}

// TestAddGitHook_ForceMoves — force=true converts the conflict to
// a Move, removing from the source list and adding to target.
func TestAddGitHook_ForceMoves(t *testing.T) {
	h := freshHook(t)
	diff, err := AddGitHook(h, ListAllow, [][]string{{"push", "--force"}}, true)
	require.NoError(t, err)
	require.Len(t, diff.HookMoved, 1)
	assert.Equal(t, "Bash(git push --force *)", diff.HookMoved[0].Rule)
	assert.Equal(t, ListDeny, diff.HookMoved[0].From)
	assert.False(t, h.Has(GitHookDeny, []string{"push", "--force"}))
	assert.True(t, h.Has(GitHookAllow, []string{"push", "--force"}))
}

// TestRemoveGitHook_Found — removing a present prefix records it
// under HookRemoved.
func TestRemoveGitHook_Found(t *testing.T) {
	h := freshHook(t)
	diff, err := RemoveGitHook(h, ListAllow, [][]string{{"show"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(git show *)"}, diff.HookRemoved)
	assert.False(t, h.Has(GitHookAllow, []string{"show"}))
}

// TestRemoveGitHook_NotInTargetIsSkipped — Remove never reaches
// into other lists; a prefix that lives in deny is Skipped when
// the user asked to remove from allow.
func TestRemoveGitHook_NotInTargetIsSkipped(t *testing.T) {
	h := freshHook(t)
	diff, err := RemoveGitHook(h, ListAllow, [][]string{{"push", "--force"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(git push --force *)"}, diff.HookSkipped)
	assert.True(t, h.Has(GitHookDeny, []string{"push", "--force"}), "still in deny")
}

// TestApply_MixedOpsRouteToBothBackends — a single Apply call with
// one git op and one non-git op mutates both files; the merged
// Diff records each in its appropriate slice.
func TestApply_MixedOpsRouteToBothBackends(t *testing.T) {
	s := freshSettings(t)
	h := freshHook(t)
	diff, err := Apply(s, h, ListAllow, []Op{
		{Kind: KindBash, Value: "ls *"},
		{Kind: KindBash, Value: "git diff *"},
	}, ApplyAdd, false)
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(ls *)"}, diff.Added)
	assert.Equal(t, []string{"Bash(git diff *)"}, diff.HookAdded)
	assert.True(t, diff.SettingsChanged())
	assert.True(t, diff.HookChanged())
}

// TestApply_HookOnlyDoesNotTouchSettings — a batch with only git
// ops leaves settings untouched (SettingsChanged returns false).
func TestApply_HookOnlyDoesNotTouchSettings(t *testing.T) {
	s := freshSettings(t)
	h := freshHook(t)
	diff, err := Apply(s, h, ListAllow, []Op{
		{Kind: KindBash, Value: "git diff *"},
	}, ApplyAdd, false)
	require.NoError(t, err)
	assert.False(t, diff.SettingsChanged())
	assert.True(t, diff.HookChanged())
	assert.Empty(t, s.List(ListAllow))
}

// TestApply_SettingsOnlyDoesNotTouchHook — a batch with no git
// ops leaves the hook file untouched (HookChanged returns false).
func TestApply_SettingsOnlyDoesNotTouchHook(t *testing.T) {
	s := freshSettings(t)
	h := freshHook(t)
	before := append([][]string(nil), h.List(GitHookAllow)...)
	diff, err := Apply(s, h, ListAllow, []Op{
		{Kind: KindBash, Value: "ls *"},
	}, ApplyAdd, false)
	require.NoError(t, err)
	assert.True(t, diff.SettingsChanged())
	assert.False(t, diff.HookChanged())
	assert.Equal(t, before, h.List(GitHookAllow))
}

// TestApply_CrossBackendConflictIsAtomic — a batch that has a
// settings conflict in op #1 and a hook conflict in op #2 returns
// both in a single ConflictError, leaving NEITHER backend mutated.
// This is the load-bearing test for the pre-flight check: a naive
// implementation that runs settings.Add then hook.Add would either
// fail half-way (partial mutation) or surface only the first
// conflict.
func TestApply_CrossBackendConflictIsAtomic(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAsk, []string{"Bash(ls *)"})
	h := freshHook(t)
	// `push --force` is already in DENY_PREFIXES per sampleHook.

	_, err := Apply(s, h, ListAllow, []Op{
		{Kind: KindBash, Value: "ls *"},
		{Kind: KindBash, Value: "git push --force *"},
	}, ApplyAdd, false)
	var conflictErr *ConflictError
	require.ErrorAs(t, err, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 2)
	// Neither store touched.
	assert.Equal(t, []string{"Bash(ls *)"}, s.List(ListAsk))
	assert.Empty(t, s.List(ListAllow))
	assert.False(t, h.Has(GitHookAllow, []string{"push", "--force"}))
	assert.True(t, h.Has(GitHookDeny, []string{"push", "--force"}))
}

// TestApply_Remove_MixedBackends — removing a mixed batch reports
// each store's removed entries in the right Diff slice.
func TestApply_Remove_MixedBackends(t *testing.T) {
	s := freshSettings(t)
	s.SetList(ListAllow, []string{"Bash(ls *)"})
	h := freshHook(t)

	diff, err := Apply(s, h, ListAllow, []Op{
		{Kind: KindBash, Value: "ls *"},
		{Kind: KindBash, Value: "show *"}, // not a git op — non-git "show"
		{Kind: KindBash, Value: "git show *"},
	}, ApplyRemove, false)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Bash(ls *)"}, diff.Removed)
	assert.ElementsMatch(t, []string{"Bash(git show *)"}, diff.HookRemoved)
	assert.False(t, h.Has(GitHookAllow, []string{"show"}))
}
