package files

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/prompt/prompttest"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/claude/sync/state/statetest"
	"github.com/Nivl/config/internal/dryrun"
)

// newMergeEnv builds a Paths with HomeDir + RepoDir as distinct
// subdirs and the state dir seeded. Same shape as settings/merge_test.go.
func newMergeEnv(t *testing.T) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	homeDir := filepath.Join(tmp, "home")
	p := state.NewPaths(configDir, homeDir)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))
	require.NoError(t, os.MkdirAll(p.HomeDir, 0o755))
	require.NoError(t, state.EnsureStateDir(p))
	return p
}

// findBackup returns the basename of the first .bkp file found
// alongside path, or "" if none exists.
func findBackup(t *testing.T, path string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	prefix := filepath.Base(path) + "."
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".bkp") {
			return e.Name()
		}
	}
	return ""
}

// TestMergeFile_Row1_AllEqualNoop — L=B=R, no I/O.
func TestMergeFile_Row1_AllEqualNoop(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"), []byte("same\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"), []byte("same\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "CLAUDE.md").Return([]byte("same\n"), nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "CLAUDE.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionNoop, r.Decision.Action)
	assert.False(t, r.Wrote)
	assert.Empty(t, findBackup(t, filepath.Join(p.HomeDir, "CLAUDE.md")))
}

// TestMergeFile_Row2_LocalEqBaseTakeRemote — L=B, R≠B; remote applied.
func TestMergeFile_Row2_LocalEqBaseTakeRemote(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"), []byte("base\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"), []byte("new\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "CLAUDE.md").Return([]byte("base\n"), nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "CLAUDE.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Decision.Action)
	assert.True(t, r.Wrote)
	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "CLAUDE.md"))
	assert.Equal(t, "new\n", string(got))
	assert.NotEmpty(t, findBackup(t, filepath.Join(p.HomeDir, "CLAUDE.md")))
}

// TestMergeFile_Row8_RemoteDeletedCleanLocalRm — L=B, R absent; rm-l with backup.
func TestMergeFile_Row8_RemoteDeletedCleanLocalRm(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "skills", "foo.md"), []byte("x"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "skills/foo.md").Return([]byte("x"), nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "skills/foo.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionRemoveLocal, r.Decision.Action)
	assert.True(t, r.Wrote)
	_, statErr := os.Stat(filepath.Join(p.HomeDir, "skills", "foo.md"))
	assert.True(t, os.IsNotExist(statErr))
	assert.NotEmpty(t, findBackup(t, filepath.Join(p.HomeDir, "skills", "foo.md")))
}

// TestMergeFile_Row13_RemoteAddCopiesIn — new file from repo; mkdir-p parent.
func TestMergeFile_Row13_RemoteAddCopiesIn(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "skills", "sub", "new.md"),
		[]byte("hello\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "skills/sub/new.md").
		Return([]byte(nil), state.ErrNoBase)

	r, err := MergeFile(context.Background(), p, fakeGit, "skills/sub/new.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Decision.Action)
	got, err := os.ReadFile(filepath.Join(p.HomeDir, "skills", "sub", "new.md"))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(got))
}

// TestMergeFile_Row5_ConflictKeepLocalNoBackup — modify-modify
// resolved keep-local creates NO backup.
func TestMergeFile_Row5_ConflictKeepLocalNoBackup(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"), []byte("local\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"), []byte("remote\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "CLAUDE.md").Return([]byte("base\n"), nil)

	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.MatchedBy(func(req prompt.Request) bool {
		return req.Kind == prompt.KindFiles && req.Key == "CLAUDE.md" &&
			strings.Contains(req.Header, "modify-modify")
	})).Return(prompt.ChoiceKeepLocal, nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "CLAUDE.md", Options{
		Prompter: fakeP,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionConflict, r.Decision.Action)
	assert.Equal(t, ConflictModifyModify, r.Decision.ConflictType)
	assert.False(t, r.Wrote)
	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "CLAUDE.md"))
	assert.Equal(t, "local\n", string(got))
	assert.Empty(t, findBackup(t, filepath.Join(p.HomeDir, "CLAUDE.md")),
		"keep-local must not create a backup")
}

// TestMergeFile_Row9_ModifyDeleteTakeRemoteRemovesLocal — modify-delete
// resolved take-remote where has_R=0 falls back to removeLocal.
func TestMergeFile_Row9_ModifyDeleteTakeRemoteRemovesLocal(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "RTK.md"), []byte("modified\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "RTK.md").Return([]byte("base\n"), nil)

	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).Return(prompt.ChoiceTakeRemote, nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "RTK.md", Options{
		Prompter: fakeP,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ConflictModifyDelete, r.Decision.ConflictType)
	assert.True(t, r.Wrote)
	_, statErr := os.Stat(filepath.Join(p.HomeDir, "RTK.md"))
	assert.True(t, os.IsNotExist(statErr))
	assert.NotEmpty(t, findBackup(t, filepath.Join(p.HomeDir, "RTK.md")))
}

// TestMergeFile_ConflictSkipHasNoFilesystemEffect — Skip is a no-op
// (HadSkip=true, no backup, no mutation).
func TestMergeFile_ConflictSkipHasNoFilesystemEffect(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"), []byte("local\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"), []byte("remote\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "CLAUDE.md").Return([]byte("base\n"), nil)

	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).Return(prompt.ChoiceSkip, nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "CLAUDE.md", Options{
		Prompter: fakeP,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, r.HadSkip)
	assert.False(t, r.Wrote)
	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "CLAUDE.md"))
	assert.Equal(t, "local\n", string(got))
	assert.Empty(t, findBackup(t, filepath.Join(p.HomeDir, "CLAUDE.md")))
}

// TestMergeFile_CopyRemoteToLocalPreservesSourceMode — when local
// already exists with one mode and remote has a different mode (e.g.
// an executable skill script), the post-copy local file MUST end up
// with the remote's mode. The pure-create path is exercised by row
// 13 (remote-add); this test exercises the overwrite path (row 2:
// L=B, R≠B).
func TestMergeFile_CopyRemoteToLocalPreservesSourceMode(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "skills", "x.sh"), []byte("old\n"), 0o600))
	// Remote is executable.
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "skills", "x.sh"), []byte("new\n"), 0o755))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "skills/x.sh").Return([]byte("old\n"), nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "skills/x.sh", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Decision.Action)
	info, err := os.Stat(filepath.Join(p.HomeDir, "skills", "x.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(),
		"remote mode (0o755) must propagate even on overwrite")
}

// TestMergeFile_CopyRemoteToLocalPreservesSourceMtime — copy
// forwards info.ModTime() from remote to local so a post-sync
// `ls -l` reflects the source mtime.
func TestMergeFile_CopyRemoteToLocalPreservesSourceMtime(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "skills", "x.md"), []byte("old\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "skills"), 0o755))
	remoteFile := filepath.Join(p.RepoDir, "skills", "x.md")
	require.NoError(t, os.WriteFile(remoteFile, []byte("new\n"), 0o644))
	// Stamp the remote with a known, deliberately-old mtime so we can
	// assert it propagated to the local copy (and didn't just happen
	// to match because of test timing).
	want := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	require.NoError(t, os.Chtimes(remoteFile, want, want))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "skills/x.md").Return([]byte("old\n"), nil)

	r, err := MergeFile(context.Background(), p, fakeGit, "skills/x.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Decision.Action)
	info, err := os.Stat(filepath.Join(p.HomeDir, "skills", "x.md"))
	require.NoError(t, err)
	// Some filesystems (e.g. FAT32) round to 2s, but tempfs/APFS/ext4
	// preserve sub-second precision. Allow 1s slop.
	assert.WithinDuration(t, want, info.ModTime(), time.Second,
		"remote mtime must propagate to local")
}

// TestFileConflictRenderer_SummaryHeadersByteCounts — verify the four
// summary lines include byte counts and the conflict type.
func TestFileConflictRenderer_SummaryHeadersByteCounts(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"),
		[]byte("local-content\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"),
		[]byte("remote\n"), 0o644))
	r := newFileConflictRenderer(context.Background(), "CLAUDE.md", []byte("base\n"),
		Presence{HasLocal: true, HasRemote: true, HasBase: true},
		ConflictModifyModify, p)
	var buf bytes.Buffer
	r.Summary(&buf)
	s := buf.String()
	assert.Contains(t, s, "base   (sha none): present (5 bytes)")
	assert.Contains(t, s, "local                : present (14 bytes)")
	assert.Contains(t, s, "remote               : present (7 bytes)")
	assert.Contains(t, s, "conflict type        : modify-modify")
}

// fakeReporter records FileChange calls for assertion. Embeds
// NullReporter so methods we don't override stay silent.
type fakeReporter struct {
	dryrun.Reporter

	fileChangeCalls []fakeFileChange
}

// fakeFileChange holds the arguments of one FileChange call.
type fakeFileChange struct {
	target  string
	before  []byte
	after   []byte
	summary string
}

// FileChange records the call.
func (f *fakeReporter) FileChange(target string, before, after []byte, summary string) {
	f.fileChangeCalls = append(f.fileChangeCalls,
		fakeFileChange{target: target, before: before, after: after, summary: summary})
}

// TestMergeFile_DryRunCopySkipsWriteReportsChange — when DryRun=true
// and ActionCopyRemoteToLocal fires, MergeFile calls reporter.FileChange
// and does NOT write the file.
func TestMergeFile_DryRunCopySkipsWriteReportsChange(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"), []byte("base\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"), []byte("new\n"), 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "CLAUDE.md").Return([]byte("base\n"), nil)

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r, err := MergeFile(context.Background(), p, fakeGit, "CLAUDE.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)
	assert.Equal(t, ActionCopyRemoteToLocal, r.Decision.Action)
	assert.False(t, r.Wrote, "DryRun: Wrote must be false")

	// File must be unchanged on disk.
	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "CLAUDE.md"))
	assert.Equal(t, "base\n", string(got), "file must not change in dry-run")

	// No backup created.
	assert.Empty(t, findBackup(t, filepath.Join(p.HomeDir, "CLAUDE.md")))

	// Reporter received exactly one FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, filepath.Join(p.HomeDir, "CLAUDE.md"), rep.fileChangeCalls[0].target)
}

// TestMergeFile_DryRunRemoveSkipsRemoveReportsChange — when DryRun=true
// and ActionRemoveLocal fires, MergeFile calls reporter.FileChange and
// does NOT remove the file.
func TestMergeFile_DryRunRemoveSkipsRemoveReportsChange(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.MkdirAll(filepath.Join(p.HomeDir, "skills"), 0o755))
	original := []byte("x")
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "skills", "foo.md"), original, 0o644))
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "skills/foo.md").Return([]byte("x"), nil)

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	r, err := MergeFile(context.Background(), p, fakeGit, "skills/foo.md", Options{
		Prompter: prompttest.NewFakePrompter(),
		DryRun:   true,
		Reporter: rep,
	})
	require.NoError(t, err)
	assert.Equal(t, ActionRemoveLocal, r.Decision.Action)
	assert.False(t, r.Wrote, "DryRun: Wrote must be false")

	// File must still exist.
	got, err := os.ReadFile(filepath.Join(p.HomeDir, "skills", "foo.md"))
	require.NoError(t, err)
	assert.Equal(t, original, got, "file must not be removed in dry-run")

	// Reporter received exactly one FileChange call (nil after = deletion).
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Nil(t, rep.fileChangeCalls[0].after)
}

// TestFileConflictRenderer_DiffShellsOutToUsrBinDiff — verify the
// renderer invokes /usr/bin/diff -u and 4-space indents the output.
func TestFileConflictRenderer_DiffShellsOutToUsrBinDiff(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "CLAUDE.md"),
		[]byte("alpha\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "CLAUDE.md"),
		[]byte("beta\n"), 0o644))

	r := newFileConflictRenderer(context.Background(), "CLAUDE.md", nil,
		Presence{HasLocal: true, HasRemote: true, HasBase: false},
		ConflictAddAddDiff, p)
	var buf bytes.Buffer
	r.Diff(&buf)
	s := buf.String()
	assert.Contains(t, s, "    ---")
	assert.Contains(t, s, "    +++")
	assert.Contains(t, s, "    -alpha")
	assert.Contains(t, s, "    +beta")
}
