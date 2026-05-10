package sync

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/symlinkfs"
)

// newSymlinkEnv builds a Paths with the repo populated for the 6 items
// and an empty home dir.
func newSymlinkEnv(t *testing.T) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	homeDir := filepath.Join(tmp, "home")
	p := state.NewPaths(configDir, homeDir)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))
	require.NoError(t, os.MkdirAll(p.HomeDir, 0o755))
	// Populate repo with the 6 items.
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"),
		[]byte("c\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "RTK.md"),
		[]byte("r\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "agents"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "commands"), 0o755))
	return p
}

// TestInstallSymlinks_FreshAllItemsCreated — on a fresh home dir, all
// 6 symlinks land in order with absolute targets.
func TestInstallSymlinks_FreshAllItemsCreated(t *testing.T) {
	p := newSymlinkEnv(t)

	require.NoError(t, InstallSymlinks(context.Background(), p, &bytes.Buffer{}, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	for _, item := range []string{"settings.json", "CLAUDE.md", "RTK.md",
		"skills", "agents", "commands"} {
		target := filepath.Join(p.HomeDir, item)
		link, err := os.Readlink(target)
		require.NoError(t, err, "symlink missing for %s", item)
		assert.Equal(t, filepath.Join(p.RepoDir, item), link,
			"link target must be absolute")
	}
}

// TestInstallSymlinks_SourceMissingSkipsSilently — a missing source
// item produces no link and no output.
func TestInstallSymlinks_SourceMissingSkipsSilently(t *testing.T) {
	p := newSymlinkEnv(t)
	require.NoError(t, os.Remove(filepath.Join(p.RepoDir, "RTK.md")))

	var out bytes.Buffer
	require.NoError(t, InstallSymlinks(context.Background(), p, &out, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	_, err := os.Lstat(filepath.Join(p.HomeDir, "RTK.md"))
	assert.True(t, os.IsNotExist(err), "missing source must not produce a link")
	assert.NotContains(t, out.String(), "RTK.md", "skip is silent")
}

// TestInstallSymlinks_IdempotentReinstall — a second run is a no-op
// when targets are already correct symlinks. No backup gets written.
func TestInstallSymlinks_IdempotentReinstall(t *testing.T) {
	p := newSymlinkEnv(t)
	require.NoError(t, InstallSymlinks(context.Background(), p, &bytes.Buffer{}, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	require.NoError(t, InstallSymlinks(context.Background(), p, &bytes.Buffer{}, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	for _, item := range symlinkItems {
		link, err := os.Readlink(filepath.Join(p.HomeDir, item))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(p.RepoDir, item), link)
	}

	entries, err := os.ReadDir(p.HomeDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp",
			"idempotent re-run must not write any .bkp file")
	}
}

// TestInstallSymlinks_BacksUpAndRelinksRegularFile — a colliding
// regular file at the target is renamed to <target>.<timestamp>.bkp
// and replaced with a symlink. No user prompt.
func TestInstallSymlinks_BacksUpAndRelinksRegularFile(t *testing.T) {
	p := newSymlinkEnv(t)
	collidingTarget := filepath.Join(p.HomeDir, "CLAUDE.md")
	require.NoError(t, os.WriteFile(collidingTarget, []byte("user-content\n"), 0o644))

	require.NoError(t, InstallSymlinks(context.Background(), p, &bytes.Buffer{}, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	// Target is now a symlink to the repo copy.
	link, err := os.Readlink(collidingTarget)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(p.RepoDir, "CLAUDE.md"), link)

	// A timestamped .bkp copy of the original lives alongside it.
	backup := findBackup(t, p.HomeDir, "CLAUDE.md")
	got, err := os.ReadFile(backup)
	require.NoError(t, err)
	assert.Equal(t, "user-content\n", string(got))
}

// TestInstallSymlinks_BacksUpAndRelinksWrongSymlink — a symlink
// pointing at the wrong source is renamed aside and replaced.
func TestInstallSymlinks_BacksUpAndRelinksWrongSymlink(t *testing.T) {
	p := newSymlinkEnv(t)
	collidingTarget := filepath.Join(p.HomeDir, "CLAUDE.md")
	other := filepath.Join(p.HomeDir, "other.md")
	require.NoError(t, os.WriteFile(other, []byte("elsewhere\n"), 0o644))
	require.NoError(t, os.Symlink(other, collidingTarget))

	require.NoError(t, InstallSymlinks(context.Background(), p, &bytes.Buffer{}, symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(collidingTarget)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(p.RepoDir, "CLAUDE.md"), link)

	// The stale symlink got backed up.
	backup := findBackup(t, p.HomeDir, "CLAUDE.md")
	backupTarget, err := os.Readlink(backup)
	require.NoError(t, err)
	assert.Equal(t, other, backupTarget)
}

// findBackup returns the absolute path of the first file in dir whose
// name starts with "<base>." and ends with ".bkp". Fails the test if
// none is found — collisions are expected to always produce exactly
// one backup per run.
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
