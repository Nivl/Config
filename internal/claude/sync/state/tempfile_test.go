package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupTempFile_AfterSuccessfulRename — close-then-rename
// already happened. Cleanup must be a no-op (no error).
func TestCleanupTempFile_AfterSuccessfulRename(t *testing.T) {
	dir := t.TempDir()
	tmp, err := os.CreateTemp(dir, "x-*.tmp")
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	final := filepath.Join(dir, "final")
	require.NoError(t, os.Rename(tmp.Name(), final))

	assert.NoError(t, CleanupTempFile(tmp))
}

// TestCleanupTempFile_OnWriteFailurePath — neither Close nor Rename
// ran. Cleanup must close the fd and remove the file.
func TestCleanupTempFile_OnWriteFailurePath(t *testing.T) {
	dir := t.TempDir()
	tmp, err := os.CreateTemp(dir, "x-*.tmp")
	require.NoError(t, err)
	tmpName := tmp.Name()

	require.NoError(t, CleanupTempFile(tmp))
	_, statErr := os.Stat(tmpName)
	assert.True(t, os.IsNotExist(statErr), "tempfile should be removed")
}

// TestCleanupTempFile_AfterExplicitClose — Close ran but Rename
// failed; file still exists. Cleanup must remove it.
func TestCleanupTempFile_AfterExplicitClose(t *testing.T) {
	dir := t.TempDir()
	tmp, err := os.CreateTemp(dir, "x-*.tmp")
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	tmpName := tmp.Name()

	require.NoError(t, CleanupTempFile(tmp))
	_, statErr := os.Stat(tmpName)
	assert.True(t, os.IsNotExist(statErr), "tempfile should be removed")
}
