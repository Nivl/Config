package perms

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVariants_BashNonGit — a non-git bash command produces a single
// Bash(value) rule and nothing else.
func TestVariants_BashNonGit(t *testing.T) {
	got, err := Variants(KindBash, "col *")
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(col *)"}, got)
}

// TestVariants_BashGit — a git command gets two variants: the raw
// form and the `git -C /*` cwd-bypass form.
func TestVariants_BashGit(t *testing.T) {
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
	got, err := Variants(KindBash, "git")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Bash(git)",
		"Bash(git -C /* )",
	}, got)
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
	got, err := Variants(KindBash, "git\tstatus *")
	require.NoError(t, err)
	assert.Len(t, got, 2, "tab separator should not break git-detection")
}
