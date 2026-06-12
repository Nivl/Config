package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/appsetup"
	"github.com/Nivl/config/internal/appsetup/appsetuptest"
	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/claude/sync"
	"github.com/Nivl/config/internal/claude/sync/state"
	"github.com/Nivl/config/internal/configgen"
	"github.com/Nivl/config/internal/configgen/configgentest"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
)

// noopBrewRunner is a brew.Runner that succeeds at everything as a
// no-op. Sufficient for the orchestration test, which only cares
// that the packages phase completes — not what was installed.
type noopBrewRunner struct{}

func (noopBrewRunner) Upgrade(context.Context, ...string) error { return nil }
func (noopBrewRunner) Install(context.Context, ...string) error { return nil }
func (noopBrewRunner) Outdated(context.Context) ([]string, error) {
	return nil, nil
}

func (noopBrewRunner) IsCaskInstalled(context.Context, string) (bool, error) {
	return false, nil
}

func (noopBrewRunner) IsAppRunning(context.Context, string) (bool, error) {
	return false, nil
}

func (noopBrewRunner) ListCaskApps(context.Context, string) ([]string, error) {
	return nil, nil
}

func (noopBrewRunner) InstallCask(context.Context, string) (brew.CaskOutcome, error) {
	return brew.CaskOutcome{Status: brew.StatusInstalled}, nil
}

// recordingSetupReporter wraps a NullReporter so the non-Section
// methods (FileChange, Shellout, etc.) stay silent. We only care
// about the sequence of Section() calls to verify phase ordering.
type recordingSetupReporter struct {
	dryrun.Reporter

	sections []string
}

func (r *recordingSetupReporter) Section(name string) {
	r.sections = append(r.sections, name)
}

// orchestrationFixture is the shared state every orchestration test
// needs: a tempdir HOME pre-populated so dotfiles/configgen/appsetup
// phases can complete without shellouts; a recording reporter; and
// a cfg with all boundary factories faked.
type orchestrationFixture struct {
	cfg       *appConfig
	reporter  *recordingSetupReporter
	appRunner *appsetuptest.FakeCmdRunner
	home      string
	configDir string
}

// makeOrchestrationFixture sets up the tempdirs, pre-populates the
// files setupCmd's late phases otherwise want to write/symlink, wires
// the GitHub-auth-success Capture mock, and returns the assembled
// cfg + recording reporter. Tests override individual factory fields
// on cfg.* to inject failures or alternative behaviour.
func makeOrchestrationFixture(t *testing.T) *orchestrationFixture {
	t.Helper()
	home := t.TempDir()
	configDir := t.TempDir()

	// dotfiles.CopyConfigFiles symlinks .emacs.d and .golangci.yml
	// from configDir/shared_config/ into $HOME; the sources must
	// exist for the symlink to succeed.
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "shared_config", ".emacs.d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "shared_config", ".golangci.yml"), []byte(""), 0o644))

	// SetupSSH skips ssh-keygen when default.pub already exists.
	// Pre-creating avoids needing an appRunner.On("ssh-keygen", ...)
	// expectation.
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".ssh"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "default"), []byte("priv"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".ssh", "default.pub"), []byte("pub"), 0o644))

	// SetupGpg's first-install branch runs killall + gpg-agent
	// --daemon. Pre-creating the conf file forces the
	// already-installed path, which skips both shellouts.
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".gnupg"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gnupg", "gpg-agent.conf"), []byte(""), 0o600))

	t.Setenv("HOME", home)

	appRunner := appsetuptest.NewFakeCmdRunner()
	// SetupGitHub's auth-status probe; returning "success" short-
	// circuits before any prompt or `gh auth login -w` invocation.
	appRunner.On("Capture", mock.Anything,
		"gh", "auth", "status", "-a", "--json", "hosts").
		Return([]byte(`{"hosts":{"github.com":[{"state":"success"}]}}`), nil)

	rec := &recordingSetupReporter{Reporter: dryrun.NewNullReporter()}
	cfg := &appConfig{
		streams:       iox.Streams{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
		reporter:      rec,
		configDir:     configDir,
		newBrewRunner: func(iox.Streams) brew.Runner { return noopBrewRunner{} },
		newClaudeSync: func(context.Context, state.Paths, sync.Options) (sync.Summary, error) {
			return sync.Summary{}, nil
		},
		newAppRunner:       func(iox.Streams) appsetup.CmdRunner { return appRunner },
		newConfiggenRunner: func() configgen.CmdRunner { return configgentest.NewFakeCmdRunner() },
	}
	return &orchestrationFixture{
		cfg:       cfg,
		reporter:  rec,
		appRunner: appRunner,
		home:      home,
		configDir: configDir,
	}
}

// orchestrationParams returns a setupParams with every prearg set
// non-empty so the userinput.* prompt steps short-circuit and the
// test doesn't block on stdin.
func orchestrationParams() setupParams {
	return setupParams{
		personalArg:    "false",
		devRoot:        "/x",
		gitOrg:         "Acme",
		gitHost:        "git@github.com",
		homebrewPrefix: "/opt/homebrew",
	}
}

// TestSetupCmd_OrchestrationOrder — happy-path phase ordering. The
// reporter.Section() calls are the canonical signal for "we entered
// this phase," so asserting on their sequence locks in the contract
// without depending on side-effect timing.
func TestSetupCmd_OrchestrationOrder(t *testing.T) {
	f := makeOrchestrationFixture(t)
	require.NoError(t, setupCmd(context.Background(), f.cfg, orchestrationParams()))

	assert.Equal(t,
		[]string{"packages", "claude sync", "dotfiles", "configgen", "appsetup"},
		f.reporter.sections,
		"phase ordering regressed")
}

// TestSetupCmd_PackagesErrorAbortsRemainingPhases — a brew error in
// the packages phase wraps with "install packages:" and prevents
// every later Section from being emitted. Verifies the
// fail-loud-fail-fast contract.
func TestSetupCmd_PackagesErrorAbortsRemainingPhases(t *testing.T) {
	f := makeOrchestrationFixture(t)
	f.cfg.newBrewRunner = func(iox.Streams) brew.Runner { return errBrewRunner{} }

	err := setupCmd(context.Background(), f.cfg, orchestrationParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "install packages:",
		"packages phase error must be wrapped with the phase name")
	assert.Equal(t, []string{"packages"}, f.reporter.sections,
		"phases after the failed one must not run")
}

// TestSetupCmd_ClaudeSyncErrorAbortsRemainingPhases — same shape,
// but the failure is one phase later. Proves dotfiles / configgen /
// appsetup don't run.
func TestSetupCmd_ClaudeSyncErrorAbortsRemainingPhases(t *testing.T) {
	f := makeOrchestrationFixture(t)
	f.cfg.newClaudeSync = func(context.Context, state.Paths, sync.Options) (sync.Summary, error) {
		return sync.Summary{}, errors.New("sync exploded")
	}

	err := setupCmd(context.Background(), f.cfg, orchestrationParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude sync: sync exploded",
		"claude sync error must be wrapped with the phase name")
	assert.Equal(t, []string{"packages", "claude sync"}, f.reporter.sections,
		"phases after claude sync must not run when it errors")
}

// errBrewRunner is a brew.Runner whose Upgrade and Outdated both error
// out. The upgrade failure limps into the failed-packages prompt, the
// prompt hits EOF on the fixture's empty stdin, and the packages phase
// fails loudly — proving a broken brew plus no interactive input still
// aborts the run.
type errBrewRunner struct{}

func (errBrewRunner) Upgrade(context.Context, ...string) error { return errors.New("brew exploded") }
func (errBrewRunner) Install(context.Context, ...string) error { return nil }
func (errBrewRunner) Outdated(context.Context) ([]string, error) {
	return nil, errors.New("brew exploded")
}

func (errBrewRunner) IsCaskInstalled(context.Context, string) (bool, error) {
	return false, nil
}

func (errBrewRunner) IsAppRunning(context.Context, string) (bool, error) {
	return false, nil
}

func (errBrewRunner) ListCaskApps(context.Context, string) ([]string, error) {
	return nil, nil
}

func (errBrewRunner) InstallCask(context.Context, string) (brew.CaskOutcome, error) {
	return brew.CaskOutcome{Status: brew.StatusInstalled}, nil
}
