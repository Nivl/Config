package sync

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/dryrun"
)

// newGitRepo creates a tempdir initialized as a git repo and
// populates .githooks/pre-commit with a stub script.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.CommandContext(context.Background(), "git", "init", "-q", dir)
	require.NoError(t, cmd.Run())
	hooksDir := filepath.Join(dir, ".githooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre-commit"),
		[]byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))
	return dir
}

// TestInstallPrecommitHook_NonGitDirPrintsWarnAndSkips — non-git
// configDir produces a warning and a clean return.
func TestInstallPrecommitHook_NonGitDirPrintsWarnAndSkips(t *testing.T) {
	dir := t.TempDir() // NOT a git repo
	var out bytes.Buffer
	err := InstallPrecommitHook(context.Background(), dir, &out, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	assert.Contains(t, out.String(), "is not a git repo, skipping")
}

// TestInstallPrecommitHook_CreatesRelativeSymlink — target absent
// case: install creates a relative symlink.
func TestInstallPrecommitHook_CreatesRelativeSymlink(t *testing.T) {
	dir := newGitRepo(t)
	var out bytes.Buffer
	err := InstallPrecommitHook(context.Background(), dir, &out, false, dryrun.NewNullReporter())
	require.NoError(t, err)
	target := filepath.Join(dir, ".git", "hooks", "pre-commit")
	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, "../../.githooks/pre-commit", link,
		"link target must be RELATIVE")
}

// TestInstallPrecommitHook_IdempotentReinstall — running twice in a
// row is a no-op on the second call.
func TestInstallPrecommitHook_IdempotentReinstall(t *testing.T) {
	dir := newGitRepo(t)
	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))

	target := filepath.Join(dir, ".git", "hooks", "pre-commit")
	info1, err := os.Lstat(target)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &out, false, dryrun.NewNullReporter()))
	info2, err := os.Lstat(target)
	require.NoError(t, err)
	assert.Equal(t, info1.ModTime(), info2.ModTime(),
		"idempotent: existing correct symlink must not be recreated")
	assert.Empty(t, out.String(), "second run is silent")
}

// TestInstallPrecommitHook_RefusesToClobberForeignSymlink — a
// pre-existing symlink pointing elsewhere is left alone with a
// warning.
func TestInstallPrecommitHook_RefusesToClobberForeignSymlink(t *testing.T) {
	dir := newGitRepo(t)
	hooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	target := filepath.Join(hooksDir, "pre-commit")
	require.NoError(t, os.Symlink("/some/other/path", target))

	var out bytes.Buffer
	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &out, false, dryrun.NewNullReporter()))
	link, _ := os.Readlink(target)
	assert.Equal(t, "/some/other/path", link, "foreign symlink left alone")
	assert.Contains(t, out.String(), "existing pre-commit symlink points elsewhere")
}

// TestInstallPrecommitHook_RefusesToClobberRegularFile — a
// pre-existing regular file is left alone with a warning.
func TestInstallPrecommitHook_RefusesToClobberRegularFile(t *testing.T) {
	dir := newGitRepo(t)
	hooksDir := filepath.Join(dir, ".git", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	target := filepath.Join(hooksDir, "pre-commit")
	require.NoError(t, os.WriteFile(target, []byte("# someone-elses-hook\n"), 0o755))

	var out bytes.Buffer
	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &out, false, dryrun.NewNullReporter()))
	got, _ := os.ReadFile(target)
	assert.Equal(t, "# someone-elses-hook\n", string(got), "regular file left alone")
	assert.Contains(t, out.String(), "existing pre-commit hook found at")
}

// TestInstallPrecommitHook_ChmodsNonExecutableSource — a regular
// non-executable .githooks/pre-commit is chmod +x'd defensively.
func TestInstallPrecommitHook_ChmodsNonExecutableSource(t *testing.T) {
	dir := newGitRepo(t)
	hookSource := filepath.Join(dir, ".githooks", "pre-commit")
	require.NoError(t, os.Chmod(hookSource, 0o644)) // strip exec bit

	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))
	info, err := os.Stat(hookSource)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o111,
		"regular non-executable hook source must be chmod +x'd")
}

// TestInstallPrecommitHook_MissingHookSourceIsTolerated — defensive
// chmod is silently skipped when the source doesn't exist (e.g.
// .githooks/ was deleted). The symlink is still created; commits
// will fail with a clear error at commit time.
func TestInstallPrecommitHook_MissingHookSourceIsTolerated(t *testing.T) {
	dir := newGitRepo(t)
	require.NoError(t, os.Remove(filepath.Join(dir, ".githooks", "pre-commit")))

	require.NoError(t, InstallPrecommitHook(context.Background(), dir, &bytes.Buffer{}, false, dryrun.NewNullReporter()))
	target := filepath.Join(dir, ".git", "hooks", "pre-commit")
	link, err := os.Readlink(target)
	require.NoError(t, err)
	assert.Equal(t, "../../.githooks/pre-commit", link)
}

// fakeReporter records FileChange and Symlink calls for assertion.
// Embeds NullReporter so methods we don't override stay silent.
type fakeReporter struct {
	dryrun.Reporter

	fileChangeCalls []fakeFileChange
	symlinkCalls    []fakeSymlink
}

// fakeFileChange holds the arguments of one FileChange call.
type fakeFileChange struct {
	target  string
	before  []byte
	after   []byte
	summary string
}

// fakeSymlink holds the arguments of one Symlink call.
type fakeSymlink struct {
	target   string
	linkTo   string
	decision string
}

// FileChange records the call.
func (f *fakeReporter) FileChange(target string, before, after []byte, summary string) {
	f.fileChangeCalls = append(f.fileChangeCalls,
		fakeFileChange{target: target, before: before, after: after, summary: summary})
}

// Symlink records the call.
func (f *fakeReporter) Symlink(target, linkTo, decision string) {
	f.symlinkCalls = append(f.symlinkCalls,
		fakeSymlink{target: target, linkTo: linkTo, decision: decision})
}

// TestInstallPrecommitHook_DryRunSkipsSymlinkReports — when dryRun=true,
// the function calls reporter.Symlink and does NOT create the symlink.
func TestInstallPrecommitHook_DryRunSkipsSymlinkReports(t *testing.T) {
	dir := newGitRepo(t)

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := InstallPrecommitHook(context.Background(), dir, &bytes.Buffer{}, true, rep)
	require.NoError(t, err)

	target := filepath.Join(dir, ".git", "hooks", "pre-commit")
	_, statErr := os.Lstat(target)
	assert.True(t, os.IsNotExist(statErr), "symlink must not be created in dry-run")

	// Reporter received one Symlink call with "would-create".
	require.Len(t, rep.symlinkCalls, 1)
	assert.Equal(t, target, rep.symlinkCalls[0].target)
	assert.Equal(t, "../../.githooks/pre-commit", rep.symlinkCalls[0].linkTo)
	assert.Equal(t, "would-create", rep.symlinkCalls[0].decision)
}
