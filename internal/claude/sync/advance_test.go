package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/claude/sync/state/statetest"
	"github.com/Nivl/config/internal/dryrun"
)

// TestAdvanceLastSyncCommit_WritesFullSHA — the function must write
// the FULL 40-char SHA returned by HeadSHA, followed by the trailing
// newline that state.WriteLastSyncSHA appends.
func TestAdvanceLastSyncCommit_WritesFullSHA(t *testing.T) {
	tmp := t.TempDir()
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("HeadSHA", mock.Anything).
		Return("abcdef1234567890abcdef1234567890abcdef12", nil)

	require.NoError(t, AdvanceLastSyncCommit(context.Background(), p, fakeGit, false, dryrun.NewNullReporter()))

	got, err := os.ReadFile(p.LastSyncFile)
	require.NoError(t, err)
	assert.Equal(t, "abcdef1234567890abcdef1234567890abcdef12\n", string(got),
		"full 40-char SHA followed by single newline")
}

// TestAdvanceLastSyncCommit_HeadSHAErrorPropagates verifies the
// function wraps and propagates errors from HeadSHA.
func TestAdvanceLastSyncCommit_HeadSHAErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("HeadSHA", mock.Anything).Return("", errors.New("git missing"))

	err := AdvanceLastSyncCommit(context.Background(), p, fakeGit, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "head sha")
}

// TestAdvanceLastSyncCommit_WriteErrorPropagates verifies the function
// wraps and propagates errors from state.WriteLastSyncSHA (e.g. when
// the state dir doesn't exist).
func TestAdvanceLastSyncCommit_WriteErrorPropagates(t *testing.T) {
	tmp := t.TempDir()
	// DELIBERATELY don't create StateDir.
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("HeadSHA", mock.Anything).
		Return("abcdef1234567890abcdef1234567890abcdef12", nil)

	err := AdvanceLastSyncCommit(context.Background(), p, fakeGit, false, dryrun.NewNullReporter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write last-sync sha")
}

// TestAdvanceLastSyncCommit_DryRunSkipsWriteReportsChange — when
// dryRun=true, the function calls reporter.FileChange and does NOT
// write the last-sync-commit file.
func TestAdvanceLastSyncCommit_DryRunSkipsWriteReportsChange(t *testing.T) {
	tmp := t.TempDir()
	p := state.NewPaths(filepath.Join(tmp, "config"), filepath.Join(tmp, "home"))
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	const headSHA = "abcdef1234567890abcdef1234567890abcdef12"
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("HeadSHA", mock.Anything).Return(headSHA, nil)

	rep := &fakeReporter{Reporter: dryrun.NewNullReporter()}
	err := AdvanceLastSyncCommit(context.Background(), p, fakeGit, true, rep)
	require.NoError(t, err)

	// File must not have been created.
	_, statErr := os.Stat(p.LastSyncFile)
	assert.True(t, os.IsNotExist(statErr), "last-sync-commit must not be created in dry-run")

	// Reporter received one FileChange call.
	require.Len(t, rep.fileChangeCalls, 1)
	assert.Equal(t, p.LastSyncFile, rep.fileChangeCalls[0].target)
	assert.Equal(t, []byte(headSHA+"\n"), rep.fileChangeCalls[0].after)
}
