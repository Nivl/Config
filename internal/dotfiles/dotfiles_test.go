package dotfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// testEnv stages a fresh tempdir as configDir with the four source
// items under shared_config/ + a separate homeDir. Returns
// (configDir, homeDir).
func testEnv(t *testing.T) (configDir, homeDir string) {
	t.Helper()
	tmp := t.TempDir()
	configDir = filepath.Join(tmp, "config")
	homeDir = filepath.Join(tmp, "home")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	for _, item := range dotfileItems {
		require.NoError(t, os.MkdirAll(filepath.Join(configDir, sharedConfigSubdir, item), 0o755))
	}
	return configDir, homeDir
}

// TestCopyConfigFiles_FreshInstall — happy path: all 4 dotfiles get
// symlinked + .emacs-saves dir created.
func TestCopyConfigFiles_FreshInstall(t *testing.T) {
	configDir, homeDir := testEnv(t)
	var out bytes.Buffer

	err := CopyConfigFiles(context.Background(), configDir, homeDir, &out, false, dryrun.NewNullReporter())
	require.NoError(t, err)

	for _, item := range dotfileItems {
		link, err := os.Readlink(filepath.Join(homeDir, item))
		require.NoError(t, err, "%s should be a symlink", item)
		assert.Equal(t, filepath.Join(configDir, sharedConfigSubdir, item), link)
	}
	info, err := os.Stat(filepath.Join(homeDir, ".emacs-saves"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestCopyConfigFiles_IdempotentReinstall — a second run is a no-op
// when targets are already correct symlinks. No backup files written.
func TestCopyConfigFiles_IdempotentReinstall(t *testing.T) {
	configDir, homeDir := testEnv(t)
	require.NoError(t, CopyConfigFiles(context.Background(), configDir, homeDir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	require.NoError(t, CopyConfigFiles(context.Background(), configDir, homeDir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	for _, item := range dotfileItems {
		link, err := os.Readlink(filepath.Join(homeDir, item))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(configDir, sharedConfigSubdir, item), link)
	}
	entries, err := os.ReadDir(homeDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp",
			"idempotent re-run must not write any .bkp file")
	}
}

// TestCopyConfigFiles_BacksUpAndRelinksRegularFile — a colliding
// regular file at the target is renamed to <target>.<timestamp>.bkp
// and replaced with a symlink. No prompt.
func TestCopyConfigFiles_BacksUpAndRelinksRegularFile(t *testing.T) {
	configDir, homeDir := testEnv(t)
	collidingTarget := filepath.Join(homeDir, ".emacs.d")
	require.NoError(t, os.WriteFile(collidingTarget, []byte("user-content\n"), 0o644))

	require.NoError(t, CopyConfigFiles(context.Background(), configDir, homeDir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	link, err := os.Readlink(collidingTarget)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, sharedConfigSubdir, ".emacs.d"), link)

	backup := findBackup(t, homeDir, ".emacs.d")
	got, err := os.ReadFile(backup)
	require.NoError(t, err)
	assert.Equal(t, "user-content\n", string(got))
}

// TestCopyConfigFiles_BacksUpAndRelinksWrongSymlink — a stale symlink
// pointing at a different source is moved aside and replaced.
func TestCopyConfigFiles_BacksUpAndRelinksWrongSymlink(t *testing.T) {
	configDir, homeDir := testEnv(t)
	collidingTarget := filepath.Join(homeDir, ".emacs.d")
	other := filepath.Join(homeDir, "other-omz")
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.Symlink(other, collidingTarget))

	require.NoError(t, CopyConfigFiles(context.Background(), configDir, homeDir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	link, err := os.Readlink(collidingTarget)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(configDir, sharedConfigSubdir, ".emacs.d"), link)

	backup := findBackup(t, homeDir, ".emacs.d")
	backupTarget, err := os.Readlink(backup)
	require.NoError(t, err)
	assert.Equal(t, other, backupTarget)
}

// TestCopyConfigFiles_EmacsSavesAlreadyExists — the .emacs-saves
// MkdirAll is idempotent.
func TestCopyConfigFiles_EmacsSavesAlreadyExists(t *testing.T) {
	configDir, homeDir := testEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".emacs-saves"), 0o755))

	require.NoError(t, CopyConfigFiles(context.Background(), configDir, homeDir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	info, err := os.Stat(filepath.Join(homeDir, ".emacs-saves"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestCopyConfigFiles_DryRunSuppressesAllWrites — under dryRun=true
// no symlinks are created, no .emacs-saves dir is mkdir'd, and the
// "Linked …" progress lines are not written. The reporter receives
// per-item Symlink decisions and a FileChange entry for the mkdir.
func TestCopyConfigFiles_DryRunSuppressesAllWrites(t *testing.T) {
	configDir, homeDir := testEnv(t)
	var out bytes.Buffer
	var stderr bytes.Buffer
	reporter := dryrun.NewReporter(&stderr)

	err := CopyConfigFiles(context.Background(), configDir, homeDir, &out, true, reporter)
	require.NoError(t, err)

	// No symlinks created.
	for _, item := range dotfileItems {
		_, statErr := os.Lstat(filepath.Join(homeDir, item))
		assert.True(t, os.IsNotExist(statErr),
			"%s must not exist in dry-run", item)
	}
	// .emacs-saves dir not created.
	_, statErr := os.Stat(filepath.Join(homeDir, ".emacs-saves"))
	assert.True(t, os.IsNotExist(statErr),
		".emacs-saves must not be mkdir'd in dry-run")

	// Progress output ("Linked …") suppressed in dry-run.
	assert.Empty(t, out.String(),
		"production progress lines must not be emitted in dry-run")

	// Reporter received the decisions.
	got := stderr.String()
	for _, item := range dotfileItems {
		assert.Contains(t, got, item, "Symlink decision for %s missing", item)
	}
	assert.Contains(t, got, ".emacs-saves")
	assert.Contains(t, got, "would mkdir")
}

// findBackup returns the absolute path of the first file in dir whose
// name starts with "<base>." and ends with ".bkp".
func findBackup(t *testing.T, dir, base string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), base+".") && strings.HasSuffix(e.Name(), ".bkp") {
			return filepath.Join(dir, e.Name())
		}
	}
	t.Fatalf("no backup file found for %s under %s", base, dir)
	return ""
}
