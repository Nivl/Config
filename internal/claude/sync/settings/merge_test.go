package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/prompt"
	"github.com/Nivl/config/internal/claude/sync/prompt/prompttest"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/claude/sync/state/statetest"
	"github.com/Nivl/config/internal/dryrun"
)

// newMergeEnv builds a tempdir-backed Paths with EnsureStateDir already
// run, plus pre-creates the home and repo .claude directories.
// configDir and homeDir are distinct subdirectories so RepoDir and HomeDir
// never alias each other.
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

// TestMerge_NoChangesReturnsClean — all three sides equal; Wrote=false.
func TestMerge_NoChangesReturnsClean(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"model":"opus"}`), nil)
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote)
	assert.False(t, r.HadSkips)
	assert.Empty(t, r.Decisions)
}

// TestMerge_FirstSyncMissingLocalFastCopy — no local file → fast-path
// copy preserves remote's exact bytes.
func TestMerge_FirstSyncMissingLocalFastCopy(t *testing.T) {
	p := newMergeEnv(t)
	remoteBytes := []byte("{\n  \"model\": \"opus\"\n}\n") // formatted
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		remoteBytes, 0o644))

	git := statetest.NewFakeGit() // no expectations: never called on fast path
	fakeP := prompttest.NewFakePrompter()

	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, r.Wrote)

	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	assert.Equal(t, remoteBytes, got, "fast-path preserves remote formatting")
}

// TestMerge_SetMergeLogLineByteIdentical asserts the exact log line
// format emitted on a set-merge take-remote.
func TestMerge_SetMergeLogLineByteIdentical(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"permissions":{"allow":["A","D"]}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"permissions":{"allow":["A","B","C"]}}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"permissions":{"allow":["A","B"]}}`), nil)
	fakeP := prompttest.NewFakePrompter() // no expectations: no conflicts

	var out bytes.Buffer
	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, r.Wrote)
	assert.Contains(t, out.String(),
		"settings.json: .permissions.allow merged: +1 -1 (now 3 items)\n")

	var got map[string]any
	gotBytes, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	perms := got["permissions"].(map[string]any)
	allow := perms["allow"].([]any)
	assert.ElementsMatch(t, []any{"A", "C", "D"}, allow)
}

// TestMerge_ConflictPromptReturnsTakeRemote applies remote when the
// prompter says take-remote.
func TestMerge_ConflictPromptReturnsTakeRemote(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"sonnet"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"haiku"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"model":"opus"}`), nil)
	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).
		Return(prompt.ChoiceTakeRemote, nil)

	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, r.Wrote)
	assert.False(t, r.HadSkips)

	gotBytes, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(gotBytes, &got)
	assert.Equal(t, "sonnet", got["model"])
	fakeP.AssertExpectations(t)
}

// TestMerge_ConflictPromptReturnsSkip leaves local untouched, HadSkips=true.
func TestMerge_ConflictPromptReturnsSkip(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"sonnet"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"haiku"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"model":"opus"}`), nil)
	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).
		Return(prompt.ChoiceSkip, nil)

	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote)
	assert.True(t, r.HadSkips)

	gotBytes, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(gotBytes, &got)
	assert.Equal(t, "haiku", got["model"], "local untouched")
}

// TestMerge_MalformedLocalJSON_WarnsAndReturnsCleanNoTouch — malformed
// local emits a warning, leaves local untouched, and creates no backup.
func TestMerge_MalformedLocalJSON_WarnsAndReturnsCleanNoTouch(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	bad := []byte(`{not json`)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		bad, 0o644))

	git := statetest.NewFakeGit()
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err, "soft outcome — no error returned")
	assert.False(t, r.Wrote)
	assert.False(t, r.HadSkips)
	assert.Contains(t, out.String(), "is not valid JSON, refusing to merge")

	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	assert.Equal(t, bad, got, "local must not be touched")

	entries, _ := os.ReadDir(p.HomeDir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp", "no backup created when refusing to merge")
	}
}

// TestMerge_MalformedRemoteJSON_WarnsAndReturnsClean — malformed
// remote emits a warning and returns clean, leaving local alone.
func TestMerge_MalformedRemoteJSON_WarnsAndReturnsClean(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{not json`), 0o644))
	original := []byte(`{"model":"opus"}`)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		original, 0o644))

	git := statetest.NewFakeGit()
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote)
	assert.Contains(t, out.String(), "is not valid JSON, skipping")

	got, _ := os.ReadFile(filepath.Join(p.HomeDir, "settings.json"))
	assert.Equal(t, original, got)
}

// TestMerge_GitFailureSilentFirstSync — ErrNoBase → treat base as {}, no warning.
func TestMerge_GitFailureSilentFirstSync(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(nil), state.ErrNoBase)
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote)
	assert.NotContains(t, out.String(), "cannot read settings.json",
		"first-sync no-anchor is silent")
}

// TestMerge_StaleLastSyncWarnsAndFallsBack — an anchor SHA is set but
// the commit lacks settings.json (ErrBaseUnreadable, the path-move /
// unreachable case). The merge falls back to an empty base AND warns,
// including the short SHA and a recovery hint.
func TestMerge_StaleLastSyncWarnsAndFallsBack(t *testing.T) {
	p := newMergeEnv(t)
	// Pre-write a SHA so baseShortSHA can surface it in the warning.
	require.NoError(t, os.WriteFile(p.LastSyncFile, []byte("deadbeef\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(nil), state.ErrBaseUnreadable)
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	_, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	got := out.String()
	assert.Contains(t, got, "settings.json not found at last-sync-commit")
	assert.Contains(t, got, "reset it to HEAD")
	assert.Contains(t, got, "deadbee", "warning should include the short anchor SHA")
}

// TestMerge_UnknownGitErrorWarns — a non-sentinel git failure (e.g. the
// git binary itself misbehaving) still warns generically and falls back
// to an empty base.
func TestMerge_UnknownGitErrorWarns(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(nil), errors.New("git: fatal: not a git repository"))
	fakeP := prompttest.NewFakePrompter()

	var out bytes.Buffer
	_, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &out,
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "cannot read settings.json at last-sync-commit")
}

// TestMerge_BackupCreatedBeforeWrite — verifies a .bkp file exists with the
// pre-merge content when any decisions are applied.
func TestMerge_BackupCreatedBeforeWrite(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"sonnet"}`), 0o644))
	original := []byte(`{"model":"opus"}`)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		original, 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"model":"opus"}`), nil)
	fakeP := prompttest.NewFakePrompter()

	_, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)

	entries, _ := os.ReadDir(p.HomeDir)
	var backup string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bkp") {
			backup = e.Name()
		}
	}
	require.NotEmpty(t, backup, "backup file must exist")
	backupBytes, _ := os.ReadFile(filepath.Join(p.HomeDir, backup))
	assert.Equal(t, original, backupBytes)
}

// TestAtomicWrite_Mode0644 — the on-disk file ends at 0o644 mode,
// matching bash's `>"$out_tmp"` umask default, not the 0o600 that
// os.CreateTemp would otherwise produce.
func TestAtomicWrite_Mode0644(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	require.NoError(t, atomicWrite(path, []byte("{}\n")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
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

// TestMerge_DryRunFastPathSkipsWriteReportsChange — when DryRun=true
// and local is absent, Merge calls reporter.FileChange and does NOT
// write the file.
func TestMerge_DryRunFastPathSkipsWriteReportsChange(t *testing.T) {
	p := newMergeEnv(t)
	remoteBytes := []byte(`{"model":"opus"}`)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		remoteBytes, 0o644))
	// No local file.

	git := statetest.NewFakeGit()
	fakeP := prompttest.NewFakePrompter()
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}

	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		DryRun: true, Reporter: rep,
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote, "DryRun: Wrote must be false")

	// Local file must not have been created.
	localPath := filepath.Join(p.HomeDir, "settings.json")
	_, statErr := os.Stat(localPath)
	assert.True(t, os.IsNotExist(statErr), "file must not be created in dry-run")

	// Reporter received exactly one FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, localPath, rep.fileChangeCalls[0].target)
}

// TestMerge_DryRunMergeSkipsWriteReportsChange — when DryRun=true
// and a clean take-remote merge is needed (no conflict), Merge calls
// reporter.FileChange and does NOT write the file or create a backup.
func TestMerge_DryRunMergeSkipsWriteReportsChange(t *testing.T) {
	p := newMergeEnv(t)
	// Remote changed from base; local stayed at base (clean take-remote, no conflict).
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"sonnet"}`), 0o644))
	original := []byte(`{"model":"opus"}`)
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		original, 0o644))

	git := statetest.NewFakeGit()
	// Base == local, so remote is applied without conflict.
	git.On("ShowBase", mock.Anything, "settings.json").
		Return(original, nil)
	fakeP := prompttest.NewFakePrompter() // no expectations: no conflicts
	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}

	r, err := Merge(context.Background(), p, git, Options{
		Prompter: fakeP, Out: &bytes.Buffer{},
		DryRun: true, Reporter: rep,
	})
	require.NoError(t, err)
	assert.False(t, r.Wrote, "DryRun: Wrote must be false")

	// Local file must be unchanged.
	localPath := filepath.Join(p.HomeDir, "settings.json")
	got, _ := os.ReadFile(localPath)
	assert.Equal(t, original, got, "file must not change in dry-run")

	// No backup created.
	entries, _ := os.ReadDir(p.HomeDir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bkp", "no backup in dry-run")
	}

	// Reporter received exactly one FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, localPath, rep.fileChangeCalls[0].target)
	assert.Equal(t, original, rep.fileChangeCalls[0].before)
}

// TestMerge_ContextCancellation surfaces ctx cancellation as a wrapped error.
func TestMerge_ContextCancellation(t *testing.T) {
	p := newMergeEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	git := statetest.NewFakeGit()
	git.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(nil), context.Canceled)
	fakeP := prompttest.NewFakePrompter()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Merge(ctx, p, git, Options{Prompter: fakeP, Out: &bytes.Buffer{}, Reporter: dryrun.NewNullReporter()})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
