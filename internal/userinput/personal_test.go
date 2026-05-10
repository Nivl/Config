package userinput

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// TestPersonal_PreargFastPathPassesThroughVerbatim — non-empty prearg
// passes through; "garbage" is returned without validation; .zprofile
// is NOT touched. The cmd layer resolves the value from --personal /
// PERSONAL_COMPUTER before invoking; this package does not read env.
func TestPersonal_PreargFastPathPassesThroughVerbatim(t *testing.T) {
	tmp := t.TempDir()
	out := &bytes.Buffer{}
	got, err := Personal("garbage", strings.NewReader(""), out, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "garbage", got)
	assert.Empty(t, out.String(), "no prompt should be printed when prearg is set")

	_, statErr := os.Stat(filepath.Join(tmp, ".zprofile"))
	assert.True(t, os.IsNotExist(statErr), ".zprofile must not be touched on prearg fast-path")
}

// TestPersonal_PromptLower — lowercase "y" → "true".
func TestPersonal_PromptLower(t *testing.T) {
	tmp := t.TempDir()
	out := &bytes.Buffer{}
	got, err := Personal("", strings.NewReader("y\n"), out, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "true", got)
	assert.Contains(t, out.String(), "Is this for a personal computer (y/n)? ")
}

// TestPersonal_PromptUpper — uppercase "Y" → "true".
func TestPersonal_PromptUpper(t *testing.T) {
	tmp := t.TempDir()
	out := &bytes.Buffer{}
	got, err := Personal("", strings.NewReader("Y\n"), out, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "true", got)
}

// TestPersonal_PromptNo — "n" → "false".
func TestPersonal_PromptNo(t *testing.T) {
	tmp := t.TempDir()
	out := &bytes.Buffer{}
	got, err := Personal("", strings.NewReader("n\n"), out, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "false", got)
}

// TestPersonal_InvalidLoops — bogus input retries until valid.
func TestPersonal_InvalidLoops(t *testing.T) {
	tmp := t.TempDir()
	out := &bytes.Buffer{}
	got, err := Personal("", strings.NewReader("bogus\nmaybe\nn\n"), out, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "false", got)
	// "Invalid value" appears twice (once per bogus answer).
	assert.Equal(t, 2, strings.Count(out.String(), "Invalid value"))
}

// TestPersonal_PromptAppendsToZprofile — Personal always ensures
// persistence via append-with-grep-guard.
func TestPersonal_PromptAppendsToZprofile(t *testing.T) {
	tmp := t.TempDir()
	_, err := Personal("", strings.NewReader("y\n"), &bytes.Buffer{}, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmp, ".zprofile"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "\nexport PERSONAL_COMPUTER=true\n")
}

// TestPersonal_AppendPreservesExistingContent — pre-existing content
// in ~/.zprofile survives the append (no truncate).
func TestPersonal_AppendPreservesExistingContent(t *testing.T) {
	tmp := t.TempDir()
	zprofile := filepath.Join(tmp, ".zprofile")
	require.NoError(t, os.WriteFile(zprofile, []byte("# user content\nexport FOO=bar\n"), 0o644))

	_, err := Personal("", strings.NewReader("y\n"), &bytes.Buffer{}, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	content, err := os.ReadFile(zprofile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# user content")
	assert.Contains(t, string(content), "export FOO=bar")
	assert.Contains(t, string(content), "export PERSONAL_COMPUTER=true")
}

// TestPersonal_AppendIdempotent — grep-guard prevents duplicate append
// on second run with same value pre-written.
func TestPersonal_AppendIdempotent(t *testing.T) {
	tmp := t.TempDir()
	zprofile := filepath.Join(tmp, ".zprofile")
	require.NoError(t, os.WriteFile(zprofile, []byte("export PERSONAL_COMPUTER=true\n"), 0o644))

	_, err := Personal("", strings.NewReader("y\n"), &bytes.Buffer{}, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	content, err := os.ReadFile(zprofile)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(content), "export PERSONAL_COMPUTER="),
		"line must not be duplicated")
}

// TestPersonal_AppendOverwritesChangedValue — when .zprofile has the
// line with a different value (e.g. =false), and the user answers
// "y", the new =true line is appended below. zsh sources top-to-
// bottom so the later assignment wins. The duplicate line is
// intentional — an in-place rewrite would risk mangling unrelated
// user edits.
func TestPersonal_AppendOverwritesChangedValue(t *testing.T) {
	tmp := t.TempDir()
	zprofile := filepath.Join(tmp, ".zprofile")
	require.NoError(t, os.WriteFile(zprofile, []byte("export PERSONAL_COMPUTER=false\n"), 0o644))

	got, err := Personal("", strings.NewReader("y\n"), &bytes.Buffer{}, tmp, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Equal(t, "true", got)

	content, err := os.ReadFile(zprofile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "export PERSONAL_COMPUTER=true",
		"new value must be appended so zsh's last-write-wins picks it up")
	// Pre-existing =false line is preserved; the new =true line wins.
	assert.Contains(t, string(content), "export PERSONAL_COMPUTER=false")
}

// TestPersonal_EOFReturnsError — closed stdin without an answer
// returns errNoInput.
func TestPersonal_EOFReturnsError(t *testing.T) {
	tmp := t.TempDir()
	_, err := Personal("", strings.NewReader(""), &bytes.Buffer{}, tmp, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

// TestPersonal_DryRunDoesNotWriteZprofile — under dryRun=true the
// prompt still runs (the value gates downstream behaviour), but the
// .zprofile append is suppressed and the would-be change is announced
// through the reporter instead.
func TestPersonal_DryRunDoesNotWriteZprofile(t *testing.T) {
	tmp := t.TempDir()
	reporter := dryrun.NewReporter(&bytes.Buffer{})
	got, err := Personal("", strings.NewReader("y\n"), &bytes.Buffer{}, tmp, true, reporter)
	require.NoError(t, err)
	assert.Equal(t, "true", got)

	_, statErr := os.Stat(filepath.Join(tmp, ".zprofile"))
	assert.True(t, os.IsNotExist(statErr), ".zprofile must not be written under dryRun")
}
