package userinput

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitOrg_PreargFastPath — non-empty prearg returns verbatim;
// no prompt.
func TestGitOrg_PreargFastPath(t *testing.T) {
	got, err := GitOrg("Acme", strings.NewReader(""), &bytes.Buffer{}, false)
	require.NoError(t, err)
	assert.Equal(t, "Acme", got)
}

// TestGitOrg_PersonalFastPath — personalComputer=true hardcodes "Nivl"
// without prompting.
func TestGitOrg_PersonalFastPath(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := GitOrg("", strings.NewReader(""), out, true)
	require.NoError(t, err)
	assert.Equal(t, "Nivl", got)
	assert.Empty(t, out.String(), "no prompt should be printed on personal fast-path")
}

// TestGitOrg_EmptyInputLoops — empty input re-prompts until non-empty.
func TestGitOrg_EmptyInputLoops(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := GitOrg("", strings.NewReader("\n\nAcme\n"), out, false)
	require.NoError(t, err)
	assert.Equal(t, "Acme", got)
	// Prompt printed 3 times (empty, empty, valid).
	assert.Equal(t, 3, strings.Count(out.String(), "What is the name of the git org? "))
}

// TestGitOrg_NonEmptyInputEchoed — user input verbatim when prearg
// unset and personalComputer=false.
func TestGitOrg_NonEmptyInputEchoed(t *testing.T) {
	got, err := GitOrg("", strings.NewReader("Acme\n"), &bytes.Buffer{}, false)
	require.NoError(t, err)
	assert.Equal(t, "Acme", got)
}

// TestGitOrg_EOFReturnsError — closed stdin while waiting on the
// non-empty retry loop returns an error.
func TestGitOrg_EOFReturnsError(t *testing.T) {
	_, err := GitOrg("", strings.NewReader(""), &bytes.Buffer{}, false)
	require.Error(t, err)
}
