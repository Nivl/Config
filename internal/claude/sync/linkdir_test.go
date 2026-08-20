package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/symlinkfs"
)

// newLinkDirEnv builds a Paths whose repo skills dir holds the named
// entries as directories, each carrying a SKILL.md. The home dir
// exists but starts empty.
func newLinkDirEnv(t *testing.T, repoSkills ...string) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills"), 0o755))
	require.NoError(t, os.MkdirAll(p.HomeDir, 0o755))
	for _, name := range repoSkills {
		dir := filepath.Join(p.RepoDir, "skills", name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"),
			[]byte("name: "+name+"\n"), 0o644))
	}
	return p
}

// linkFor returns the readlink of ~/.claude/skills/<name>.
func linkFor(t *testing.T, p state.Paths, name string) string {
	t.Helper()
	link, err := os.Readlink(filepath.Join(p.HomeDir, "skills", name))
	require.NoError(t, err)
	return link
}

// liveOpts is the non-dry-run InstallOpts used by most cases here.
func liveOpts() symlinkfs.InstallOpts {
	return symlinkfs.InstallOpts{Reporter: dryrun.NewNullReporter()}
}

// TestLinkDirEntries_FreshLinksEveryRepoEntry — an empty home dir gets
// one symlink per repo entry, each pointing at the absolute repo path,
// and each reported on the progress writer.
func TestLinkDirEntries_FreshLinksEveryRepoEntry(t *testing.T) {
	p := newLinkDirEnv(t, "in-depth-review", "work-on")

	var out bytes.Buffer
	require.NoError(t, LinkDirEntries(p, "skills", &out, liveOpts()))

	for _, name := range []string{"in-depth-review", "work-on"} {
		source := filepath.Join(p.RepoDir, "skills", name)
		assert.Equal(t, source, linkFor(t, p, name))
		assert.Contains(t, out.String(),
			"Linked "+filepath.Join(p.HomeDir, "skills", name)+" -> "+source+"\n",
			"each new link must be reported target-first")
	}
}

// TestLinkDirEntries_CreatesHomeDirWhenMissing — the local dir is
// mkdir -p'd before any linking.
func TestLinkDirEntries_CreatesHomeDirWhenMissing(t *testing.T) {
	p := newLinkDirEnv(t, "rpe")
	require.NoError(t, os.RemoveAll(p.HomeDir))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	assert.Equal(t, filepath.Join(p.RepoDir, "skills", "rpe"), linkFor(t, p, "rpe"))
}

// TestLinkDirEntries_ReplacesCopiedDirectoryWithoutBackup — a copied
// skill dir left over from copy mode is deleted, not renamed to a
// .bkp sibling that Claude Code would read as a duplicate skill.
func TestLinkDirEntries_ReplacesCopiedDirectoryWithoutBackup(t *testing.T) {
	p := newLinkDirEnv(t, "in-depth-review")
	stale := filepath.Join(p.HomeDir, "skills", "in-depth-review")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("stale\n"), 0o644))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	assert.Equal(t, filepath.Join(p.RepoDir, "skills", "in-depth-review"),
		linkFor(t, p, "in-depth-review"))

	entries, err := os.ReadDir(filepath.Join(p.HomeDir, "skills"))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp",
			"a backup inside skills/ would register as a duplicate skill")
	}
}

// TestLinkDirEntries_AlreadyCorrectLinkIsNoOp — a re-run over correct
// links neither errors nor reports progress.
func TestLinkDirEntries_AlreadyCorrectLinkIsNoOp(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	var out bytes.Buffer
	require.NoError(t, LinkDirEntries(p, "skills", &out, liveOpts()))

	assert.Equal(t, filepath.Join(p.RepoDir, "skills", "work-on"), linkFor(t, p, "work-on"))
	assert.Empty(t, out.String(), "an unchanged link emits no progress line")
}

// TestLinkDirEntries_LeavesEntriesAbsentFromRepoAlone — local-only
// skills and hand-made symlinks to other repos survive untouched.
func TestLinkDirEntries_LeavesEntriesAbsentFromRepoAlone(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	homeSkills := filepath.Join(p.HomeDir, "skills")
	require.NoError(t, os.MkdirAll(filepath.Join(homeSkills, "graphify"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(homeSkills, "graphify", "SKILL.md"),
		[]byte("local only\n"), 0o644))
	elsewhere := filepath.Join(t.TempDir(), "databricks-backend")
	require.NoError(t, os.MkdirAll(elsewhere, 0o755))
	require.NoError(t, os.Symlink(elsewhere, filepath.Join(homeSkills, "databricks-backend")))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	body, err := os.ReadFile(filepath.Join(homeSkills, "graphify", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "local only\n", string(body))
	assert.Equal(t, elsewhere, linkFor(t, p, "databricks-backend"))
}

// TestLinkDirEntries_PrunesDanglingRepoLink — a link into the repo
// skills dir whose target is gone (skill deleted upstream) is removed
// rather than left dangling.
func TestLinkDirEntries_PrunesDanglingRepoLink(t *testing.T) {
	p := newLinkDirEnv(t, "work-on", "retired")
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))
	require.NoError(t, os.RemoveAll(filepath.Join(p.RepoDir, "skills", "retired")))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	_, err := os.Lstat(filepath.Join(p.HomeDir, "skills", "retired"))
	assert.True(t, os.IsNotExist(err), "dangling repo link must be pruned")
	assert.Equal(t, filepath.Join(p.RepoDir, "skills", "work-on"), linkFor(t, p, "work-on"))
}

// TestLinkDirEntries_KeepsDanglingLinkToElsewhere — pruning is scoped
// to links into the repo dir; a broken link the user made themselves
// is theirs to fix.
func TestLinkDirEntries_KeepsDanglingLinkToElsewhere(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	ghost := filepath.Join(t.TempDir(), "ghost")
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	require.NoError(t, os.Symlink(ghost, filepath.Join(p.HomeDir, "skills", "mine")))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	assert.Equal(t, ghost, linkFor(t, p, "mine"))
}

// TestLinkDirEntries_SkipsDotEntries — .gitkeep, .DS_Store and friends
// in the repo dir are not linked.
func TestLinkDirEntries_SkipsDotEntries(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	repoSkills := filepath.Join(p.RepoDir, "skills")
	require.NoError(t, os.WriteFile(filepath.Join(repoSkills, ".gitkeep"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoSkills, ".DS_Store"), []byte("x"), 0o644))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	for _, name := range []string{".gitkeep", ".DS_Store"} {
		_, err := os.Lstat(filepath.Join(p.HomeDir, "skills", name))
		assert.True(t, os.IsNotExist(err), "%s must not be linked", name)
	}
}

// TestLinkDirEntries_MissingRepoDirIsNoOp — no repo dir means nothing
// to link, and no error.
func TestLinkDirEntries_MissingRepoDirIsNoOp(t *testing.T) {
	p := newLinkDirEnv(t)
	require.NoError(t, os.RemoveAll(filepath.Join(p.RepoDir, "skills")))

	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	_, err := os.Stat(filepath.Join(p.HomeDir, "skills"))
	require.NoError(t, err, "the local dir is still created")
}

// TestLinkDirEntries_IgnoresLeftoverStagingEntry — a staging entry
// stranded by a failed cleanup must survive a rerun untouched and
// unreported. Both scans already skip it, the entry loop because it
// only walks repo names and the prune because it only takes symlinks,
// and this pins that so a change to either scan cannot quietly start
// treating it as a real entry.
func TestLinkDirEntries_IgnoresLeftoverStagingEntry(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))

	stranded := filepath.Join(p.HomeDir, "skills", ".work-on.20260513023045.replacing")
	require.NoError(t, os.MkdirAll(stranded, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stranded, "SKILL.md"), []byte("stale\n"), 0o644))

	var out bytes.Buffer
	require.NoError(t, LinkDirEntries(p, "skills", &out, liveOpts()))

	assert.Equal(t, filepath.Join(p.RepoDir, "skills", "work-on"), linkFor(t, p, "work-on"))
	body, err := os.ReadFile(filepath.Join(stranded, "SKILL.md"))
	require.NoError(t, err, "the leftover must survive untouched")
	assert.Equal(t, "stale\n", string(body))
	assert.Empty(t, out.String(), "a leftover is not an entry, so it is never reported")
}

// TestLinkDirEntries_MkdirFailureIsWrapped — a local dir that cannot
// be created surfaces as a wrapped mkdir error rather than a bare one.
func TestLinkDirEntries_MkdirFailureIsWrapped(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	// A regular file where ~/.claude should be makes MkdirAll of the
	// skills subdir fail with ENOTDIR.
	require.NoError(t, os.RemoveAll(p.HomeDir))
	require.NoError(t, os.WriteFile(p.HomeDir, []byte("not a dir"), 0o644))

	err := LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mkdir local")
}

// TestLinkDirEntries_InspectTargetFailureIsWrapped — a local dir that
// cannot be looked into surfaces as a wrapped inspect error. Distinct
// from a missing target, which is simply "not linked yet".
func TestLinkDirEntries_InspectTargetFailureIsWrapped(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))
	localDir := filepath.Join(p.HomeDir, "skills")
	// Denying traversal makes the lstat inside fail with EACCES rather
	// than ENOENT, which is the branch that wraps.
	require.NoError(t, os.Chmod(localDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(localDir, 0o755) })

	err := LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inspect target")
}

// TestLinkDirEntries_RemoteReadFailureIsWrapped — a repo dir that is
// not a directory surfaces as a wrapped read error. Distinct from the
// missing-dir case, which is a silent no-op.
func TestLinkDirEntries_RemoteReadFailureIsWrapped(t *testing.T) {
	p := newLinkDirEnv(t)
	repoSkills := filepath.Join(p.RepoDir, "skills")
	require.NoError(t, os.RemoveAll(repoSkills))
	require.NoError(t, os.WriteFile(repoSkills, []byte("not a dir"), 0o644))

	err := LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read remote")
}

// TestLinkDirEntries_DryRunReportsMissingLocalDir — a dry run over an
// absent local dir reports the mkdir it would do and creates nothing.
// The sibling dry-run case starts from an existing dir, so it only
// ever exercises the no-op branch.
func TestLinkDirEntries_DryRunReportsMissingLocalDir(t *testing.T) {
	p := newLinkDirEnv(t, "work-on")
	localDir := filepath.Join(p.HomeDir, "skills")

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, symlinkfs.InstallOpts{
		Reporter: rep,
		DryRun:   true,
	}))

	_, err := os.Stat(localDir)
	assert.True(t, os.IsNotExist(err), "dry run must not create the local dir")

	var summaries []string
	for _, call := range rep.fileChangeCalls {
		if call.target == localDir {
			summaries = append(summaries, call.summary)
		}
	}
	assert.Contains(t, summaries, "would mkdir 0o755")
}

// TestLinkDirEntries_DryRunWritesNothing — a dry run reports the
// decisions and leaves the filesystem alone.
func TestLinkDirEntries_DryRunWritesNothing(t *testing.T) {
	p := newLinkDirEnv(t, "work-on", "retired")
	require.NoError(t, LinkDirEntries(p, "skills", &bytes.Buffer{}, liveOpts()))
	require.NoError(t, os.RemoveAll(filepath.Join(p.RepoDir, "skills", "retired")))
	require.NoError(t, os.RemoveAll(filepath.Join(p.HomeDir, "skills", "work-on")))
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills", "work-on"), 0o755))

	var out bytes.Buffer
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	require.NoError(t, LinkDirEntries(p, "skills", &out, symlinkfs.InstallOpts{
		Reporter: rep,
		DryRun:   true,
	}))

	info, err := os.Lstat(filepath.Join(p.HomeDir, "skills", "work-on"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "dry run must not replace the copied dir")
	dangling := filepath.Join(p.HomeDir, "skills", "retired")
	_, err = os.Lstat(dangling)
	require.NoError(t, err, "dry run must not prune the dangling link")
	assert.Empty(t, out.String(), "dry-run output belongs to the reporter")

	// Skipping the removal is only half of it. The prune has to say it
	// would have happened, or the preview under-reports.
	var summaries []string
	for _, call := range rep.fileChangeCalls {
		if call.target == dangling {
			summaries = append(summaries, call.summary)
		}
	}
	assert.Contains(t, summaries, "would remove dangling symlink")

	// The forced Replace has to reach the preview as well, otherwise a
	// dry run would describe a backup the real run never makes.
	collided := filepath.Join(p.HomeDir, "skills", "work-on")
	var reported bool
	for _, call := range rep.symlinkCalls {
		if call.target == collided {
			assert.Equal(t, "would-replace", call.decision)
			reported = true
		}
	}
	assert.True(t, reported, "the collision must be reported")
}
