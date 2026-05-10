package state

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadLastSyncSHA_MissingFile returns empty string with no error
// — first-sync semantics: empty SHA → caller falls back to empty
// base.
func TestReadLastSyncSHA_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	sha, err := ReadLastSyncSHA(p)
	require.NoError(t, err)
	assert.Empty(t, sha)
}

// TestReadLastSyncSHA_EmptyFile returns empty string, not the empty
// file's content.
func TestReadLastSyncSHA_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))
	require.NoError(t, os.WriteFile(p.LastSyncFile, []byte(""), 0o644))

	sha, err := ReadLastSyncSHA(p)
	require.NoError(t, err)
	assert.Empty(t, sha)
}

// TestReadLastSyncSHA_PopulatedReturnsTrimmedSHA reads a SHA with trailing
// newline (the format git rev-parse HEAD produces) and returns it without
// the newline.
func TestReadLastSyncSHA_PopulatedReturnsTrimmedSHA(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))
	require.NoError(t, os.WriteFile(p.LastSyncFile,
		[]byte("abcdef1234567890abcdef1234567890abcdef12\n"), 0o644))

	sha, err := ReadLastSyncSHA(p)
	require.NoError(t, err)
	assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12", sha)
}

// TestWriteLastSyncSHA_Format writes the SHA followed by a single
// newline. Atomic write via tempfile + rename.
func TestWriteLastSyncSHA_Format(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	sha := "abcdef1234567890abcdef1234567890abcdef12"
	require.NoError(t, WriteLastSyncSHA(p, sha))

	got, err := os.ReadFile(p.LastSyncFile)
	require.NoError(t, err)
	assert.Equal(t, sha+"\n", string(got))
}

// TestWriteLastSyncSHA_Mode0644 — file ends up at 0o644, not the
// 0o600 that os.CreateTemp would otherwise produce.
func TestWriteLastSyncSHA_Mode0644(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	require.NoError(t, WriteLastSyncSHA(p, "abc"))
	info, err := os.Stat(p.LastSyncFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
