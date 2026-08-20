package sync

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

// newSyncEnv creates a tempdir that is also a real git repo, with
// configDir/.claude populated and a .githooks/pre-commit stub. Sync
// installs the hook in BOTH modes, so the stub must always exist.
func newSyncEnv(t *testing.T) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, "config")
	homeDir := filepath.Join(tmp, "home")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, exec.CommandContext(context.Background(), "git", "init", "-q", configDir).Run())
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, ".githooks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, ".githooks", "pre-commit"),
		[]byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))
	p := state.NewPaths(configDir, homeDir)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))
	return p
}

// TestSync_MissingRepoDirNoopWithWarn — missing .claude/ in the repo
// returns a no-op summary plus a warning to out.
func TestSync_MissingRepoDirNoopWithWarn(t *testing.T) {
	tmp := t.TempDir()
	p := state.NewPaths(tmp, tmp)
	// Deliberately DO NOT create p.RepoDir.

	var out bytes.Buffer
	summary, err := Sync(context.Background(), p, Options{
		Mode: ModeCopy, Out: &out, Prompter: prompttest.NewFakePrompter(),
		NewGit:   func(state.Paths) state.Git { return statetest.NewFakeGit() },
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.False(t, summary.HadSkips)
	assert.Contains(t, out.String(), "does not exist, skipping")
}

// TestSync_PrecommitInstalledInBothModes verifies the precommit hook
// is symlinked into .git/hooks/pre-commit regardless of mode.
func TestSync_PrecommitInstalledInBothModes(t *testing.T) {
	cases := []struct {
		name string
		mode Mode
	}{
		{"copy", ModeCopy},
		{"symlink", ModeSymlink},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newSyncEnv(t)
			// Populate the repo with at least one item so symlink mode has
			// something to do (and copy mode has empty merges).
			require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
				[]byte(`{}`), 0o644))

			fakeGit := statetest.NewFakeGit()
			fakeGit.On("ShowBase", mock.Anything, mock.Anything).
				Return([]byte(nil), state.ErrNoBase)
			fakeGit.On("ListTree", mock.Anything, mock.Anything).
				Return([]string(nil), nil)
			fakeGit.On("HeadSHA", mock.Anything).
				Return("abcdef1234567890abcdef1234567890abcdef12", nil).Maybe()

			_, err := Sync(context.Background(), p, Options{
				Mode:     tc.mode,
				Out:      &bytes.Buffer{},
				Prompter: prompttest.NewFakePrompter(),
				NewGit:   func(state.Paths) state.Git { return fakeGit },
				Reporter: dryrun.NewNullReporter(),
			})
			require.NoError(t, err)
			link, err := os.Readlink(filepath.Join(p.ConfigDir, ".git", "hooks", "pre-commit"))
			require.NoError(t, err, "precommit hook must be installed in %s mode", tc.name)
			assert.Equal(t, "../../.githooks/pre-commit", link)
		})
	}
}

// TestSync_SymlinkModeSkipsStateDirAndAdvance — symlink mode does NOT
// create .sync-state/ and does NOT write last-sync-commit.
func TestSync_SymlinkModeSkipsStateDirAndAdvance(t *testing.T) {
	p := newSyncEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{}`), 0o644))

	_, err := Sync(context.Background(), p, Options{
		Mode:     ModeSymlink,
		Out:      &bytes.Buffer{},
		Prompter: prompttest.NewFakePrompter(),
		NewGit:   func(state.Paths) state.Git { return statetest.NewFakeGit() },
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	_, err = os.Stat(p.StateDir)
	assert.True(t, os.IsNotExist(err), "no .sync-state in symlink mode")
	_, err = os.Stat(p.LastSyncFile)
	assert.True(t, os.IsNotExist(err), "no last-sync-commit in symlink mode")
}

// TestSync_CopyModeAdvancesOnCleanRun — after a clean copy-mode run
// (no skips), last-sync-commit holds the HEAD SHA.
func TestSync_CopyModeAdvancesOnCleanRun(t *testing.T) {
	p := newSyncEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"opus"}`), 0o644))

	const headSHA = "abcdef1234567890abcdef1234567890abcdef12"
	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, mock.Anything).
		Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ListTree", mock.Anything, mock.Anything).
		Return([]string(nil), nil)
	fakeGit.On("HeadSHA", mock.Anything).Return(headSHA, nil)

	_, err := Sync(context.Background(), p, Options{
		Mode:     ModeCopy,
		Out:      &bytes.Buffer{},
		Prompter: prompttest.NewFakePrompter(),
		NewGit:   func(state.Paths) state.Git { return fakeGit },
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	got, err := os.ReadFile(p.LastSyncFile)
	require.NoError(t, err)
	assert.Equal(t, headSHA+"\n", string(got))
}

// TestSync_CopyModeLinksSkillsAndMergesAgents — in copy mode, a skill
// arrives as a symlink into the repo while an agents file is still
// copied through the merge engine. The whole point of splitting
// linkedDirs out of mergedDirs.
func TestSync_CopyModeLinksSkillsAndMergesAgents(t *testing.T) {
	p := newSyncEnv(t)
	skillDir := filepath.Join(p.RepoDir, "skills", "in-depth-review")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("name: in-depth-review\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(p.RepoDir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "agents", "reviewer.md"),
		[]byte("agent\n"), 0o644))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, mock.Anything).
		Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ListTree", mock.Anything, mock.Anything).
		Return([]string(nil), nil)
	fakeGit.On("HeadSHA", mock.Anything).
		Return("abcdef1234567890abcdef1234567890abcdef12", nil)

	_, err := Sync(context.Background(), p, Options{
		Mode:     ModeCopy,
		Out:      &bytes.Buffer{},
		Prompter: prompttest.NewFakePrompter(),
		NewGit:   func(state.Paths) state.Git { return fakeGit },
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)

	link, err := os.Readlink(filepath.Join(p.HomeDir, "skills", "in-depth-review"))
	require.NoError(t, err, "a skill must be linked, not copied")
	assert.Equal(t, skillDir, link)

	agent := filepath.Join(p.HomeDir, "agents", "reviewer.md")
	info, err := os.Lstat(agent)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink, "agents stay on the copy path")
	body, err := os.ReadFile(agent)
	require.NoError(t, err)
	assert.Equal(t, "agent\n", string(body))
}

// TestSync_CopyModeSkipsAdvanceOnHadSkips — a skip during settings
// merge holds last-sync-commit unchanged and emits the stderr
// warning.
func TestSync_CopyModeSkipsAdvanceOnHadSkips(t *testing.T) {
	p := newSyncEnv(t)
	require.NoError(t, os.MkdirAll(p.HomeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(p.RepoDir, "settings.json"),
		[]byte(`{"model":"sonnet"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(p.HomeDir, "settings.json"),
		[]byte(`{"model":"haiku"}`), 0o644))

	fakeGit := statetest.NewFakeGit()
	fakeGit.On("ShowBase", mock.Anything, "settings.json").
		Return([]byte(`{"model":"opus"}`), nil)
	fakeGit.On("ShowBase", mock.Anything, mock.Anything).
		Return([]byte(nil), state.ErrNoBase)
	fakeGit.On("ListTree", mock.Anything, mock.Anything).
		Return([]string(nil), nil)
	// HeadSHA must NOT be called when hadSkips is true. If it is, the
	// expectation below would fail — but we don't register one, which is
	// the right testify/mock idiom for "must not be called".

	fakeP := prompttest.NewFakePrompter()
	fakeP.On("Resolve", mock.Anything, mock.Anything).Return(prompt.ChoiceSkip, nil)

	var out bytes.Buffer
	summary, err := Sync(context.Background(), p, Options{
		Mode:     ModeCopy,
		Out:      &out,
		Prompter: fakeP,
		NewGit:   func(state.Paths) state.Git { return fakeGit },
		Reporter: dryrun.NewNullReporter(),
	})
	require.NoError(t, err)
	assert.True(t, summary.HadSkips)
	assert.Contains(t, out.String(),
		"claude_setup: leaving last-sync-commit unchanged due to skipped conflicts")
	_, err = os.Stat(p.LastSyncFile)
	assert.True(t, os.IsNotExist(err),
		"last-sync-commit must be unchanged when HadSkips=true")
}
