package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/brew/brewtest"
	"github.com/Nivl/config/internal/dryrun"
	"github.com/Nivl/config/internal/iox"
	"github.com/Nivl/config/internal/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// makeTestCfg returns an appConfig with the brew runner factory
// replaced by one returning the given FakeRunner. The caller-supplied
// stdin feeds the failed-packages prompt; the stdout and stderr
// writers are wired into cfg.streams so test assertions can inspect
// progress output (stdout) and prompt output (stderr) separately.
func makeTestCfg(fake *brewtest.FakeRunner, stdin io.Reader, stdout, stderr io.Writer) *appConfig {
	return &appConfig{
		cwd:           "",
		configDir:     "",
		streams:       iox.Streams{In: stdin, Out: stdout, Err: stderr},
		newBrewRunner: func(iox.Streams) brew.Runner { return fake },
		reporter:      dryrun.NewNullReporter(),
	}
}

// expectAllInstallsSucceed wires up every brew call to succeed cleanly.
func expectAllInstallsSucceed(fake *brewtest.FakeRunner) {
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)
}

// expectNoOutdatedCasks stubs the cask-discovery pass to report nothing
// system-wide, so the run only processes the configured casks. Optional
// (unlimited, not asserted), like the packages-layer twin.
func expectNoOutdatedCasks(fake *brewtest.FakeRunner) {
	fake.On("OutdatedCasks", mock.Anything).Return(nil, nil)
}

// TestInstallPackagesCmd_Success asserts a clean run returns no error
// and AssertExpectations is satisfied on the fake. The empty stdin
// proves no prompt is read when nothing failed.
func TestInstallPackagesCmd_Success(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllInstallsSucceed(fake)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader(""), &stdout, io.Discard)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: ""})
	require.NoError(t, err)
	fake.AssertExpectations(t)
}

// TestInstallPackagesCmd_FailureAbortChoice asserts that answering the
// failed-packages prompt with 1 surfaces packages.ErrAborted, that the
// failure list reached the prompt stream (stderr), and that the
// skipped-casks trailer still prints on the abort path — those casks
// were really skipped, abort or not.
func TestInstallPackagesCmd_FailureAbortChoice(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, "docker").
		Return(brew.CaskOutcome{Status: brew.StatusSkipped}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout, stderr bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader("1\n"), &stdout, &stderr)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: ""})
	require.Error(t, err)
	require.ErrorIs(t, err, packages.ErrAborted)
	assert.Contains(t, stderr.String(), "Failed cask installs or upgrades:")
	assert.Contains(t, stderr.String(), "zoom: synthetic")
	assert.Contains(t, stdout.String(), "Skipped cask updates because the app is running:")
	// The tab-dash shape only appears in PrintSkipped's item lines —
	// the bare name also occurs in an unconditional progress line.
	assert.Contains(t, stdout.String(), "\t- docker")
}

// TestInstallPackagesCmd_FailureIgnoreChoice asserts that answering
// the prompt with 3 exits cleanly: the user accepted the failures.
func TestInstallPackagesCmd_FailureIgnoreChoice(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout, stderr bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader("3\n"), &stdout, &stderr)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: ""})
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Ignoring the failed packages and continuing.")
	assert.Contains(t, stderr.String(), "zoom: synthetic")
	// The prompt loop already listed the failures — the post-ignore
	// trailer must not repeat them.
	assert.NotContains(t, stdout.String(), "Failed cask installs or upgrades:")
}

// TestInstallPackagesCmd_FailureRetrySucceeds asserts that answering 2
// re-attempts only the failed cask, and a clean retry ends the run
// without further prompts.
func TestInstallPackagesCmd_FailureRetrySucceeds(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader("2\n"), &stdout, io.Discard)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: ""})
	require.NoError(t, err)
	// Initial pass touches every cask, the retry re-attempts only the
	// failed one — the exact count pins the retry scope.
	fake.AssertNumberOfCalls(t, "InstallCask", len(packages.CommonCasks)+len(packages.BetaCasks)+1)
	fake.AssertExpectations(t)
}

// TestInstallPackagesCmd_PersonalParamIncludesPersonalCasks asserts
// that installPackagesParams{personal: true} flips the personal-cask
// gate. The flag/env resolution happens in newInstallPackagesCmd.RunE
// (covered by the resolve* tests in root_test.go); here we exercise
// the body directly with the already-resolved bool.
func TestInstallPackagesCmd_PersonalParamIncludesPersonalCasks(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	// Two Install calls expected (formulae + fonts); collapsed by mock.Anything.
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	// Any cask install (Common, Beta, AND Personal) succeeds.
	var personalCalled bool
	fake.On("InstallCask", mock.Anything, mock.MatchedBy(func(c string) bool {
		if slices.Contains(packages.PersonalCasks, c) {
			personalCalled = true
		}
		return true
	})).Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader(""), &stdout, io.Discard)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: true, failureResolution: ""})
	require.NoError(t, err)
	assert.True(t, personalCalled, "personal=true should have included PersonalCasks")
}

// TestInstallPackagesCmd_FormulaFailureIsolatedAndPrompted asserts a
// formula group failure is contained: the run isolates the broken
// formula, finishes the casks, and the prompt decides the outcome.
func TestInstallPackagesCmd_FormulaFailureIsolatedAndPrompted(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, packages.Formulae).
		Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{packages.Formulae[0]}).
		Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout, stderr bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader("3\n"), &stdout, &stderr)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: ""})
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Failed formula installs:")
	assert.Contains(t, stderr.String(), packages.Formulae[0]+": network down")
}

// TestInstallPackagesCmd_FailureResolutionIgnoreRunsUnattended asserts
// the pre-resolved ignore answer lets a run with failures finish with
// exit 0 and an untouched stdin — the scripted escape hatch for the
// failed-packages gate.
func TestInstallPackagesCmd_FailureResolutionIgnoreRunsUnattended(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectNoOutdatedCasks(fake)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout, stderr bytes.Buffer
	cfg := makeTestCfg(fake, strings.NewReader(""), &stdout, &stderr)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false, failureResolution: "ignore"})
	require.NoError(t, err)
	assert.NotContains(t, stderr.String(), "Choice (1/2/3)?")
}

// TestValidateInstallFailureResolution covers the closed set: empty
// and the three answers pass, anything else is rejected with the bad
// value named.
func TestValidateInstallFailureResolution(t *testing.T) {
	for _, v := range []string{"", "abort", "retry", "ignore"} {
		require.NoError(t, validateInstallFailureResolution(v), "should accept %q", v)
	}
	err := validateInstallFailureResolution("yolo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yolo")
}
