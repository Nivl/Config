package perms

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// disablePathResolution swaps lookPath for a "not found" stub so the
// structural-shape tests stay deterministic regardless of the test
// machine's PATH. The previous lookPath is restored via t.Cleanup.
func disablePathResolution(t *testing.T) {
	t.Helper()
	restore := SetLookPath(func(string) (string, error) {
		return "", errors.New("disabled in test")
	})
	t.Cleanup(restore)
}

// stubLookPath swaps lookPath for a deterministic table lookup. Any
// command not in the table behaves as if not found in PATH.
func stubLookPath(t *testing.T, table map[string]string) {
	t.Helper()
	restore := SetLookPath(func(name string) (string, error) {
		if path, ok := table[name]; ok {
			return path, nil
		}
		return "", errors.New("not found in stub table")
	})
	t.Cleanup(restore)
}

// TestVariants_BashNonGit — a non-git bash command produces a single
// Bash(value) rule and nothing else (with path resolution disabled).
func TestVariants_BashNonGit(t *testing.T) {
	disablePathResolution(t)
	got, err := Variants(KindBash, "col *")
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(col *)"}, got)
}

// TestVariants_BashGit — a git command gets two variants with path
// resolution disabled: the raw form and the `git -C /*` cwd-bypass.
func TestVariants_BashGit(t *testing.T) {
	disablePathResolution(t)
	got, err := Variants(KindBash, "git status *")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(git status *)",
		"Bash(git -C /* status *)",
	}, got)
}

// TestVariants_BashGitLookalikesAreNotGit — only an exact first-token
// match of "git" triggers the cwd-bypass variant; `git-foo`, `mygit`,
// and compound commands fall back to the single-variant path.
func TestVariants_BashGitLookalikesAreNotGit(t *testing.T) {
	disablePathResolution(t)
	for _, value := range []string{"git-foo bar", "mygit status", "cd /repo && git status"} {
		got, err := Variants(KindBash, value)
		require.NoError(t, err)
		assert.Len(t, got, 1, "value %q should produce 1 variant", value)
	}
}

// TestVariants_BashSingleTokenGit — `git` with no arguments is still
// the git command (rest becomes ""). The rendered rule tolerates the
// empty rest by emitting "git -C /* " with a trailing space — that's
// fine for the matcher, which treats trailing whitespace as nothing.
func TestVariants_BashSingleTokenGit(t *testing.T) {
	disablePathResolution(t)
	got, err := Variants(KindBash, "git")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(git)",
		"Bash(git -C /* )",
	}, got)
}

// TestVariants_BashNonGitResolvesPath — when lookPath returns an
// absolute path for a non-git command's first token, a second
// Bash(<resolved> <rest>) variant is appended.
func TestVariants_BashNonGitResolvesPath(t *testing.T) {
	stubLookPath(t, map[string]string{"mv": "/bin/mv"})
	got, err := Variants(KindBash, "mv *")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(mv *)",
		"Bash(/bin/mv *)",
	}, got)
}

// TestVariants_BashGitResolvesPath — git path resolution emits the
// resolved-path twins of BOTH the raw and the cwd-bypass forms, for
// four variants total.
func TestVariants_BashGitResolvesPath(t *testing.T) {
	stubLookPath(t, map[string]string{"git": "/opt/homebrew/bin/git"})
	got, err := Variants(KindBash, "git status *")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(git status *)",
		"Bash(git -C /* status *)",
		"Bash(/opt/homebrew/bin/git status *)",
		"Bash(/opt/homebrew/bin/git -C /* status *)",
	}, got)
}

// TestVariants_BashSingleTokenResolvesPath — single-token commands
// (no `rest`) still get a resolved-path variant; the resolved value
// is just the path with no trailing space.
func TestVariants_BashSingleTokenResolvesPath(t *testing.T) {
	stubLookPath(t, map[string]string{"ls": "/bin/ls"})
	got, err := Variants(KindBash, "ls")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(ls)",
		"Bash(/bin/ls)",
	}, got)
}

// TestVariants_BashResolvedPathEqualsInputIsSkipped — if the first
// token is already an absolute path that resolves to itself,
// bashVariants must NOT emit a duplicate.
func TestVariants_BashResolvedPathEqualsInputIsSkipped(t *testing.T) {
	stubLookPath(t, map[string]string{"/bin/mv": "/bin/mv"})
	got, err := Variants(KindBash, "/bin/mv *")
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(/bin/mv *)"}, got,
		"resolved-path twin must not duplicate the input")
}

// TestVariants_BashLookPathFailureSilentlySkipped — when lookPath
// returns an error (shell builtin, unknown command), the resolved
// variant is silently omitted; the unresolved variants still ship.
func TestVariants_BashLookPathFailureSilentlySkipped(t *testing.T) {
	disablePathResolution(t)
	got, err := Variants(KindBash, "cd /repo && git status")
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(cd /repo && git status)"}, got)
}

// TestVariants_Read — Read kind always produces exactly one rule.
func TestVariants_Read(t *testing.T) {
	got, err := Variants(KindRead, "file.txt")
	require.NoError(t, err)
	assert.Equal(t, []string{"Read(file.txt)"}, got)
}

// TestVariants_Fetch — Fetch kind always produces a single
// `WebFetch(domain:<value>)` rule (the `domain:` prefix is part of
// the existing settings.json convention).
func TestVariants_Fetch(t *testing.T) {
	got, err := Variants(KindFetch, "npm.io")
	require.NoError(t, err)
	assert.Equal(t, []string{"WebFetch(domain:npm.io)"}, got)
}

// TestVariants_Skill — Skill kind produces a single Skill(<value>)
// rule. Values typically take the `<plugin>:<skill>` form (the
// plugin-namespaced shape Claude Code uses for plugin skills), but
// the expansion is opaque: whatever string the user passes goes
// verbatim into the parens.
func TestVariants_Skill(t *testing.T) {
	got, err := Variants(KindSkill, "code-review:code-review")
	require.NoError(t, err)
	assert.Equal(t, []string{"Skill(code-review:code-review)"}, got)
}

// TestVariants_EmptyValueErrors — every kind rejects an empty value
// (after trim). Empty values almost always mean a flag-parsing bug
// and should never silently expand into rules.
func TestVariants_EmptyValueErrors(t *testing.T) {
	for _, kind := range []Kind{KindBash, KindRead, KindFetch, KindSkill} {
		_, err := Variants(kind, "")
		require.Error(t, err)
		_, err = Variants(kind, "   ")
		require.Error(t, err)
	}
}

// TestVariants_TrimsSurroundingWhitespace — leading/trailing
// whitespace on the user-supplied value is trimmed before expansion
// so `--bash ' git status '` behaves the same as `--bash 'git status'`.
func TestVariants_TrimsSurroundingWhitespace(t *testing.T) {
	got, err := Variants(KindBash, "  git status *  ")
	require.NoError(t, err)
	assert.Contains(t, got, "Bash(git status *)")
	assert.NotContains(t, got, "Bash(  git status *  )")
}

// TestVariants_BashGitWithNonAsciiSpaceStillTriggersGitVariants — if
// a user pastes a value with a non-ASCII whitespace separator (e.g.
// a literal tab, vertical tab, or newline from a shell heredoc),
// splitFirstToken still recognizes the first token as "git" and
// emits the 2-variant expansion. Aligning with the upstream
// strings.TrimSpace (which uses unicode.IsSpace) keeps these
// boundaries consistent.
func TestVariants_BashGitWithNonAsciiSpaceStillTriggersGitVariants(t *testing.T) {
	disablePathResolution(t)
	got, err := Variants(KindBash, "git\tstatus *")
	require.NoError(t, err)
	assert.Len(t, got, 2, "tab separator should not break git-detection")
}
