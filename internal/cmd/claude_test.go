package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// makeClaudeTestCfg returns an appConfig with a scriptable newClaudeSync
// and an stderr buffer wired into cfg.streams.Err so tests can inspect
// the diagnostic output claudeSyncCmd produces. Stdout is io.Discard
// (sync never writes to stdout by design) and stdin is an empty reader.
func makeClaudeTestCfg(syncFn func(context.Context, state.Paths, sync.Options) (sync.Summary, error), stderr io.Writer) *appConfig {
	return &appConfig{
		streams:       iox.Streams{In: strings.NewReader(""), Out: io.Discard, Err: stderr},
		newClaudeSync: syncFn,
		reporter:      dryrun.NewNullReporter(),
	}
}

// TestClaudeSyncCmd_CleanRun — returns nil on a clean Copy-mode sync
// with no diagnostic output on stderr. (Stdout-cleanliness is now
// type-system-enforced: claudeSyncCmd doesn't take a stdout writer at
// all.)
func TestClaudeSyncCmd_CleanRun(t *testing.T) {
	var stderr bytes.Buffer
	cfg := makeClaudeTestCfg(func(_ context.Context, _ state.Paths, _ sync.Options) (sync.Summary, error) {
		return sync.Summary{}, nil
	}, &stderr)
	err := claudeSyncCmd(context.Background(), cfg, claudeSyncParams{})
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty on a clean Copy-mode sync (no conflicts, no skips)")
}

// TestClaudeSyncCmd_WithSkipsExitsZero — HadSkips is not an error.
func TestClaudeSyncCmd_WithSkipsExitsZero(t *testing.T) {
	var stderr bytes.Buffer
	cfg := makeClaudeTestCfg(func(_ context.Context, _ state.Paths, _ sync.Options) (sync.Summary, error) {
		return sync.Summary{HadSkips: true}, nil
	}, &stderr)
	err := claudeSyncCmd(context.Background(), cfg, claudeSyncParams{})
	require.NoError(t, err)
}

// TestClaudeSyncCmd_CatastrophicErrorPropagates — any non-nil error from
// Sync surfaces as the returned error.
func TestClaudeSyncCmd_CatastrophicErrorPropagates(t *testing.T) {
	var stderr bytes.Buffer
	cfg := makeClaudeTestCfg(func(_ context.Context, _ state.Paths, _ sync.Options) (sync.Summary, error) {
		return sync.Summary{}, errors.New("git missing")
	}, &stderr)
	err := claudeSyncCmd(context.Background(), cfg, claudeSyncParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git missing")
}

// TestClaudeSyncCmd_PersonalParamSetsSymlinkMode — claudeSyncParams.personal
// switches the mode to Symlink. The flag/env resolution happens in RunE
// (tested elsewhere); claudeSyncCmd itself only sees the resolved bool.
func TestClaudeSyncCmd_PersonalParamSetsSymlinkMode(t *testing.T) {
	var stderr bytes.Buffer
	var capturedMode sync.Mode
	cfg := makeClaudeTestCfg(func(_ context.Context, _ state.Paths, opts sync.Options) (sync.Summary, error) {
		capturedMode = opts.Mode
		return sync.Summary{}, nil
	}, &stderr)
	require.NoError(t, claudeSyncCmd(context.Background(), cfg, claudeSyncParams{personal: true}))
	assert.Equal(t, sync.ModeSymlink, capturedMode)
}

// TestClaudeSyncCmd_ConfigDirResolvedFromHome — uses $HOME/.melvin/config
// when cfg.configDir is empty.
func TestClaudeSyncCmd_ConfigDirResolvedFromHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	var stderr bytes.Buffer
	var capturedPaths state.Paths
	cfg := makeClaudeTestCfg(func(_ context.Context, p state.Paths, _ sync.Options) (sync.Summary, error) {
		capturedPaths = p
		return sync.Summary{}, nil
	}, &stderr)
	require.NoError(t, claudeSyncCmd(context.Background(), cfg, claudeSyncParams{}))
	assert.Equal(t, filepath.Join(tmp, ".melvin", "config", "shared_config", ".claude"), capturedPaths.RepoDir)
}

// TestValidateMergeResolution_AcceptsClosedSet — only the three documented
// values pass; everything else (including the empty string fallback case
// handled separately) is gated above.
func TestValidateMergeResolution_AcceptsClosedSet(t *testing.T) {
	for _, v := range validMergeResolutions {
		require.NoError(t, validateMergeResolution(v), "should accept %q", v)
	}
}

// TestValidateMergeResolution_EmptyMeansUnset — empty is the cobra
// default; treat as "no flag" so env fallback applies.
func TestValidateMergeResolution_EmptyMeansUnset(t *testing.T) {
	require.NoError(t, validateMergeResolution(""))
}

// TestValidateMergeResolution_RejectsUnknown — bogus values fail fast
// at flag-parse time so the user sees a clear error before any sync work.
func TestValidateMergeResolution_RejectsUnknown(t *testing.T) {
	err := validateMergeResolution("yolo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yolo")
}
