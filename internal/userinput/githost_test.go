package userinput

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitHost_PreargFastPath — non-empty prearg returns verbatim;
// menu not shown.
func TestGitHost_PreargFastPath(t *testing.T) {
	got, err := GitHost("git@custom.example.com", strings.NewReader(""), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "git@custom.example.com", got)
}

// TestGitHost_GitHub — option 1 returns the GitHub SSH host.
func TestGitHost_GitHub(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := GitHost("", strings.NewReader("1\n"), out)
	require.NoError(t, err)
	assert.Equal(t, "git@github.com", got)
	assert.Contains(t, out.String(), "Pick the git server")
	assert.Contains(t, out.String(), "1) GitHub")
	assert.Contains(t, out.String(), "2) Bitbucket")
	assert.Contains(t, out.String(), "3) Gitlab")
	assert.Contains(t, out.String(), "4) Custom")
}

// TestGitHost_Bitbucket — option 2 returns the Bitbucket SSH host.
func TestGitHost_Bitbucket(t *testing.T) {
	got, err := GitHost("", strings.NewReader("2\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "git@bitbucket.org", got)
}

// TestGitHost_Gitlab — option 3 returns the GitLab SSH host.
func TestGitHost_Gitlab(t *testing.T) {
	got, err := GitHost("", strings.NewReader("3\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "git@gitlab.com", got)
}

// TestGitHost_CustomFallsThrough — option 4 prompts for a free-form
// custom value and returns it.
func TestGitHost_CustomFallsThrough(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := GitHost("", strings.NewReader("4\ngit@scm.example.com\n"), out)
	require.NoError(t, err)
	assert.Equal(t, "git@scm.example.com", got)
	assert.Contains(t, out.String(), "Type the URL of the git server (usually in the form of git@github.com):")
}

// TestGitHost_InvalidLoops — invalid input prints "Invalid value" and
// re-prompts.
func TestGitHost_InvalidLoops(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := GitHost("", strings.NewReader("9\nx\n2\n"), out)
	require.NoError(t, err)
	assert.Equal(t, "git@bitbucket.org", got)
	assert.Equal(t, 2, strings.Count(out.String(), "Invalid value"))
}

// TestGitHost_EOFInMenuReturnsError — closed stdin while reading the
// menu returns an error.
func TestGitHost_EOFInMenuReturnsError(t *testing.T) {
	_, err := GitHost("", strings.NewReader(""), &bytes.Buffer{})
	require.Error(t, err)
}
