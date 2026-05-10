package userinput

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDevRoot_PreargFastPath — non-empty prearg returns verbatim;
// no prompt.
func TestDevRoot_PreargFastPath(t *testing.T) {
	got, err := DevRoot("/custom/path", strings.NewReader(""), &bytes.Buffer{}, "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "/custom/path", got)
}

// TestDevRoot_EmptyInputUsesDefault — empty input falls back to
// homeDir/dev.
func TestDevRoot_EmptyInputUsesDefault(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := DevRoot("", strings.NewReader("\n"), out, "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "/home/u/dev", got)
	assert.Contains(t, out.String(), "Where do you want your dev root to be? [/home/u/dev]: ")
}

// TestDevRoot_NonEmptyInputEchoed — user input verbatim.
func TestDevRoot_NonEmptyInputEchoed(t *testing.T) {
	got, err := DevRoot("", strings.NewReader("/opt/work\n"), &bytes.Buffer{}, "/home/u")
	require.NoError(t, err)
	assert.Equal(t, "/opt/work", got)
}

// TestDevRoot_EOFReturnsError — closed stdin while reading the
// dev-root prompt returns an error.
func TestDevRoot_EOFReturnsError(t *testing.T) {
	_, err := DevRoot("", strings.NewReader(""), &bytes.Buffer{}, "/home/u")
	require.Error(t, err)
}
