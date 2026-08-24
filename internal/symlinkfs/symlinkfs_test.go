package symlinkfs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// fixedNow returns a deterministic timestamp so backup filenames
// inside the tests below are predictable.
func fixedNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02 15:04:05", "2026-05-13 02:30:45")
	require.NoError(t, err)
	return ts
}

// TestInstall_FreshSymlink creates the symlink when nothing exists at
// target.
func TestInstall_FreshSymlink(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("payload"), 0o644))
	target := filepath.Join(tmp, "dst")

	require.NoError(t, Install(source, target, fixedNow(t), InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)
}

// TestInstall_IdempotentWhenAlreadyLinked — target is already a
// symlink to source; Install is a no-op (no backup file written).
func TestInstall_IdempotentWhenAlreadyLinked(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("payload"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.Symlink(source, target))

	require.NoError(t, Install(source, target, fixedNow(t), InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	// No backup file got written.
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp", "no backup should be written on idempotent re-run")
	}
}

// TestInstall_BackupsAndRelinksWhenSymlinkPointsElsewhere — target is
// a symlink but to a different source; Install moves it aside and
// creates the right symlink.
func TestInstall_BackupsAndRelinksWhenSymlinkPointsElsewhere(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("new"), 0o644))
	other := filepath.Join(tmp, "other")
	require.NoError(t, os.WriteFile(other, []byte("old"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.Symlink(other, target))

	now := fixedNow(t)
	require.NoError(t, Install(source, target, now, InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	backup := target + "." + now.Format(BackupTimestampFormat) + ".bkp"
	_, err = os.Lstat(backup)
	require.NoError(t, err, "backup symlink should exist")
}

// TestInstall_BackupsAndRelinksRegularFile — target is a regular file;
// gets renamed to the timestamped .bkp and replaced with a symlink.
func TestInstall_BackupsAndRelinksRegularFile(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("new"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.WriteFile(target, []byte("user-content"), 0o644))

	now := fixedNow(t)
	require.NoError(t, Install(source, target, now, InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	backup := target + "." + now.Format(BackupTimestampFormat) + ".bkp"
	body, err := os.ReadFile(backup)
	require.NoError(t, err)
	assert.Equal(t, "user-content", string(body), "backup must preserve the original bytes")
}

// TestInstall_BackupsAndRelinksDirectory — target is a directory; gets
// renamed atomically (the directory itself moves, contents preserved).
func TestInstall_BackupsAndRelinksDirectory(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("new"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "nested", "file.txt"), []byte("inside"), 0o644))

	now := fixedNow(t)
	require.NoError(t, Install(source, target, now, InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	backup := target + "." + now.Format(BackupTimestampFormat) + ".bkp"
	body, err := os.ReadFile(filepath.Join(backup, "nested", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "inside", string(body), "backup directory must preserve contents")
}

// TestInstall_BackupsAndRelinksBrokenSymlink — target is a symlink to
// a nonexistent source; still gets backed up and re-linked. (This
// is the case where Lstat succeeds but readlink reveals a dangling
// pointer.)
func TestInstall_BackupsAndRelinksBrokenSymlink(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("new"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.Symlink(filepath.Join(tmp, "ghost"), target))

	now := fixedNow(t)
	require.NoError(t, Install(source, target, now, InstallOpts{Reporter: dryrun.NewNullReporter()}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	backup := target + "." + now.Format(BackupTimestampFormat) + ".bkp"
	_, err = os.Lstat(backup)
	require.NoError(t, err, "broken symlink should still be backed up")
}

// TestInstall_BackupFilenameFormat asserts the exact suffix format —
// 14 digits (YYYYMMDDHHmmSS, 24-hour clock) then .bkp.
func TestInstall_BackupFilenameFormat(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("new"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	ts, err := time.Parse("2006-01-02 15:04:05", "2026-12-31 23:59:59")
	require.NoError(t, err)
	require.NoError(t, Install(source, target, ts, InstallOpts{Reporter: dryrun.NewNullReporter()}))

	expected := target + ".20261231235959.bkp"
	_, err = os.Stat(expected)
	require.NoError(t, err, "backup filename must match <target>.YYYYMMDDHHmmSS.bkp 24h")
}

// TestInstall_ReplaceRemovesCollidingDirectory — with Replace set, a
// colliding directory is deleted outright instead of renamed to a
// .bkp sibling.
func TestInstall_ReplaceRemovesCollidingDirectory(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(source, 0o755))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "nested", "file.txt"), []byte("stale"), 0o644))

	now := fixedNow(t)
	require.NoError(t, Install(source, target, now, InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp", "Replace must not write a backup")
	}
}

// TestInstall_ReplaceRelinksSymlinkPointingElsewhere — the collision
// shape the backup path already covers, checked for Replace too. An
// existing link to a different source is repointed and leaves no
// backup or staging leftover behind.
func TestInstall_ReplaceRelinksSymlinkPointingElsewhere(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(source, 0o755))
	other := filepath.Join(tmp, "other")
	require.NoError(t, os.MkdirAll(other, 0o755))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.Symlink(other, target))

	require.NoError(t, Install(source, target, fixedNow(t), InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp")
		assert.NotContains(t, e.Name(), ".replacing")
	}
	// The link was replaced, not the directory it pointed at.
	_, err = os.Stat(other)
	require.NoError(t, err, "Replace must not follow the old link and delete its target")
}

// TestInstall_ReplaceFailsWhenMoveAsideFails — the first thing replace
// does can fail too. A parent that denies writes blocks the move, so
// nothing is touched and the error names the step.
func TestInstall_ReplaceFailsWhenMoveAsideFails(t *testing.T) {
	// This test forces the rename to fail by chmodding the parent
	// directory to 0o555 so the move-aside is denied. Root ignores
	// directory permissions, so under root the rename still succeeds
	// and the require.Error below never fires.
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the directory permissions this test uses to force the failure")
	}
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(source, 0o755))
	// The parent is its own directory rather than tmp, so restoring it
	// cannot race t.TempDir's teardown.
	parent := filepath.Join(tmp, "parent")
	require.NoError(t, os.MkdirAll(parent, 0o755))
	target := filepath.Join(parent, "dst")
	require.NoError(t, os.WriteFile(target, []byte("user-content"), 0o644))
	require.NoError(t, os.Chmod(parent, 0o555))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	err := Install(source, target, fixedNow(t), InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rename")
	body, readErr := os.ReadFile(target)
	require.NoError(t, readErr, "a failed move must leave the target alone")
	assert.Equal(t, "user-content", string(body))
}

// TestInstall_ReplaceRelinksRegularFile — the third collision shape.
// The backup path covers a regular file; Replace drops its bytes
// instead of preserving them under a .bkp name.
func TestInstall_ReplaceRelinksRegularFile(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(source, 0o755))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.WriteFile(target, []byte("user-content"), 0o644))

	require.NoError(t, Install(source, target, fixedNow(t), InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)

	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp", "Replace keeps no copy of the bytes")
		assert.NotContains(t, e.Name(), ".replacing")
	}
}

// TestInstall_ReplaceKeepsLinkWhenCleanupFails — Replace installs the
// link before deleting the old content, so a delete that fails still
// leaves the caller with a correct symlink rather than a hole where
// the target used to be. The unremovable child is what forces the
// delete to fail.
func TestInstall_ReplaceKeepsLinkWhenCleanupFails(t *testing.T) {
	// This test forces the cleanup to fail by chmodding a
	// subdirectory to 0o555 so RemoveAll cannot remove its children.
	// Root ignores permission bits, so under root that RemoveAll
	// still succeeds and the require.Error below never fires.
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the directory permissions this test uses to force the failure")
	}
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.MkdirAll(source, 0o755))
	target := filepath.Join(tmp, "dst")
	locked := filepath.Join(target, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "file.txt"), []byte("x"), 0o644))
	// A read-only directory refuses to give up its children, so
	// RemoveAll fails partway. Restore the mode or t.TempDir's own
	// cleanup hits the same wall. Install moves the content to a
	// staging path, so rather than recompute where that landed, walk
	// what is left and reopen every directory.
	require.NoError(t, os.Chmod(locked, 0o555))
	t.Cleanup(func() {
		_ = filepath.WalkDir(tmp, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr == nil && d.IsDir() {
				_ = os.Chmod(path, 0o755)
			}
			return nil
		})
	})

	err := Install(source, target, fixedNow(t), InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	})
	require.Error(t, err, "a failed cleanup must still be reported")

	link, readErr := os.Readlink(target)
	require.NoError(t, readErr, "the symlink must be in place even though cleanup failed")
	assert.Equal(t, source, link)

	// Whatever the failed delete left behind must be hidden. A visible
	// leftover beside the link is what Replace callers use Replace to
	// avoid, since their directory gets scanned for real entries.
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	for _, e := range entries {
		if e.Name() == filepath.Base(source) || e.Name() == filepath.Base(target) {
			continue
		}
		assert.True(t, strings.HasPrefix(e.Name(), "."),
			"leftover %q must be dot-prefixed", e.Name())
	}
}

// TestInstall_ReplaceIdempotentWhenAlreadyLinked — Replace does not
// disturb a target that already points at source.
func TestInstall_ReplaceIdempotentWhenAlreadyLinked(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	require.NoError(t, os.WriteFile(source, []byte("payload"), 0o644))
	target := filepath.Join(tmp, "dst")
	require.NoError(t, os.Symlink(source, target))

	require.NoError(t, Install(source, target, fixedNow(t), InstallOpts{
		Reporter: dryrun.NewNullReporter(),
		Replace:  true,
	}))

	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, source, link)
}

// fakeReporter records calls for assertion. Embeds NullReporter so
// methods we don't override stay silent.
type fakeReporter struct {
	dryrun.Reporter

	symlinkCalls []fakeSymlink
}

// fakeSymlink holds the arguments of a single Symlink call.
type fakeSymlink struct {
	target, linkTo, decision string
}

// Symlink records the call.
func (f *fakeReporter) Symlink(target, linkTo, decision string) {
	f.symlinkCalls = append(f.symlinkCalls,
		fakeSymlink{target: target, linkTo: linkTo, decision: decision})
}

// TestInstall_DryRunFreshReportsWouldCreate — fresh install in
// dry-run mode reports "would-create" and creates no link.
func TestInstall_DryRunFreshReportsWouldCreate(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	target := filepath.Join(tmp, "tgt")
	require.NoError(t, os.WriteFile(source, []byte("x"), 0o644))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Install(source, target, time.Now(), InstallOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	// No symlink was created.
	_, statErr := os.Lstat(target)
	assert.True(t, os.IsNotExist(statErr))

	require.Len(t, rep.symlinkCalls, 1)
	assert.Equal(t, "would-create", rep.symlinkCalls[0].decision)
	assert.Equal(t, source, rep.symlinkCalls[0].linkTo)
	assert.Equal(t, target, rep.symlinkCalls[0].target)
}

// TestInstall_DryRunCorrectSymlinkReportsNoOp — when target already
// points at source, dry-run reports "no-op".
func TestInstall_DryRunCorrectSymlinkReportsNoOp(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	target := filepath.Join(tmp, "tgt")
	require.NoError(t, os.WriteFile(source, []byte("x"), 0o644))
	require.NoError(t, os.Symlink(source, target))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Install(source, target, time.Now(), InstallOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	require.Len(t, rep.symlinkCalls, 1)
	assert.Equal(t, "no-op", rep.symlinkCalls[0].decision)
}

// TestInstall_DryRunCollisionReportsBackup — colliding non-symlink
// target reports "would-back-up-then-create" and changes nothing on
// disk.
func TestInstall_DryRunCollisionReportsBackup(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	target := filepath.Join(tmp, "tgt")
	require.NoError(t, os.WriteFile(source, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(target, []byte("user-content"), 0o644))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Install(source, target, time.Now(), InstallOpts{
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)

	// Target file is unchanged.
	got, _ := os.ReadFile(target)
	assert.Equal(t, "user-content", string(got))

	require.Len(t, rep.symlinkCalls, 1)
	assert.Equal(t, "would-back-up-then-create", rep.symlinkCalls[0].decision)
}

// TestInstall_DryRunReplaceReportsWouldReplace — colliding target
// under Replace reports "would-replace" and leaves disk untouched.
func TestInstall_DryRunReplaceReportsWouldReplace(t *testing.T) {
	tmp := t.TempDir()
	source := filepath.Join(tmp, "src")
	target := filepath.Join(tmp, "tgt")
	require.NoError(t, os.MkdirAll(source, 0o755))
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "keep.txt"), []byte("intact"), 0o644))

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := Install(source, target, fixedNow(t), InstallOpts{
		DryRun:   true,
		Reporter: rep,
		Replace:  true,
	})
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(target, "keep.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "intact", string(got))

	require.Len(t, rep.symlinkCalls, 1)
	assert.Equal(t, "would-replace", rep.symlinkCalls[0].decision)
}
