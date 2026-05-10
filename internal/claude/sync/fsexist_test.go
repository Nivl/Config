package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirExists — directory present → true; missing path → false;
// regular file at the path (not a directory) → false. The three
// branches cover the full truth table for the helper that gates
// dry-run "would mkdir" reporting in sync.go.
func TestDirExists(t *testing.T) {
	tmp := t.TempDir()

	t.Run("existing directory", func(t *testing.T) {
		assert.True(t, dirExists(tmp))
	})
	t.Run("missing path", func(t *testing.T) {
		assert.False(t, dirExists(filepath.Join(tmp, "nope")))
	})
	t.Run("path is a regular file", func(t *testing.T) {
		file := filepath.Join(tmp, "afile")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
		assert.False(t, dirExists(file),
			"a regular file must not satisfy dirExists, even though stat succeeds")
	})
}

// TestFileExists — symmetric to TestDirExists: regular file → true;
// missing → false; directory at the path → false. Symlinks are
// followed by os.Stat, so a symlink to a regular file is true.
func TestFileExists(t *testing.T) {
	tmp := t.TempDir()

	t.Run("existing regular file", func(t *testing.T) {
		file := filepath.Join(tmp, "afile")
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
		assert.True(t, fileExists(file))
	})
	t.Run("missing path", func(t *testing.T) {
		assert.False(t, fileExists(filepath.Join(tmp, "nope")))
	})
	t.Run("path is a directory", func(t *testing.T) {
		assert.False(t, fileExists(tmp),
			"a directory must not satisfy fileExists")
	})
}
