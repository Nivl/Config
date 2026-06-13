package packages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"testing"

	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/brew/brewtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// expectAllFormulaeInstalls sets up FakeRunner expectations for the four
// successful formula install groups. Used by tests where formulae are
// expected to succeed and we want to focus assertions on cask behavior.
func expectAllFormulaeInstalls(fake *brewtest.FakeRunner) {
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(nil).Once()
	fake.On("Install", mock.Anything, Fonts).Return(nil).Once()
	fake.On("Install", mock.Anything, DevTools).Return(nil).Once()
	fake.On("Install", mock.Anything, AI).Return(nil).Once()
}

// expectAllCasksInstall sets up FakeRunner expectations for every cask in
// the given list to install cleanly (not running, no failure).
func expectAllCasksInstall(fake *brewtest.FakeRunner, casks []string) {
	for _, c := range casks {
		fake.On("InstallCask", mock.Anything, c).
			Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	}
}

// TestInstall_FreshInstall_AllCasksProceed is the Go-side version of bash
// scenario 1 (tests/install_cask_update_test.sh). All formulae install,
// all common and beta casks install cleanly. Personal casks are excluded
// because Opts.Personal is false.
func TestInstall_FreshInstall_AllCasksProceed(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllFormulaeInstalls(fake)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{Personal: false})
	require.NoError(t, err)
	assert.Empty(t, summary.Skipped)
	assert.False(t, summary.HasFailures())
	// The fake's .Once() expectations already prove every cask was
	// requested exactly once — no Installed count to cross-check.
	fake.AssertExpectations(t)
}

// TestInstall_DockerRunning_Skipped asserts that when InstallCask reports
// StatusSkipped, the cask goes into summary.Skipped and the run continues.
func TestInstall_DockerRunning_Skipped(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllFormulaeInstalls(fake)
	for _, c := range CommonCasks {
		if c == "docker" {
			fake.On("InstallCask", mock.Anything, "docker").
				Return(brew.CaskOutcome{Status: brew.StatusSkipped}, nil).Once()
			continue
		}
		fake.On("InstallCask", mock.Anything, c).
			Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	}
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	assert.Equal(t, []string{"docker"}, summary.Skipped)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstall_CaskFailureCapturesReason asserts that StatusFailed with a
// Reason populates summary.FailedCasks verbatim.
func TestInstall_CaskFailureCapturesReason(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllFormulaeInstalls(fake)
	for _, c := range CommonCasks {
		if c == "raycast" {
			fake.On("InstallCask", mock.Anything, "raycast").Return(brew.CaskOutcome{
				Status: brew.StatusFailed,
				Reason: "Error: raycast download failed",
			}, nil).Once()
			continue
		}
		fake.On("InstallCask", mock.Anything, c).
			Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	}
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Equal(t, "raycast", summary.FailedCasks[0].Name)
	assert.Equal(t, "Error: raycast download failed", summary.FailedCasks[0].Reason)
	fake.AssertExpectations(t)
}

// TestInstall_FormulaFailureIsolatesPerFormula asserts that a formula
// group failure is contained: every formula in the group is
// re-attempted alone, the broken one lands in FailedFormulae, and the
// cask phase still runs.
func TestInstall_FormulaFailureIsolatesPerFormula(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(errors.New("network down")).Once()
	// Every other isolated formula and the remaining groups succeed.
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedFormulae, 1)
	assert.Equal(t, Formulae[0], summary.FailedFormulae[0].Name)
	assert.Equal(t, "network down", summary.FailedFormulae[0].Reason)
	fake.AssertExpectations(t)
}

// TestInstall_UpgradeFailureCollectsOutdated asserts that a failed bulk
// upgrade does not abort: the still-outdated packages land in
// FailedUpgrades and the rest of the run proceeds.
func TestInstall_UpgradeFailureCollectsOutdated(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"the-unarchiver"}, nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedUpgrades, 1)
	assert.Equal(t, "the-unarchiver", summary.FailedUpgrades[0].Name)
	assert.True(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstall_UpgradeFailureNothingOutdated asserts that a non-zero
// `brew upgrade` with nothing left outdated (e.g. a cleanup hiccup) is
// treated as success — there is nothing actionable to retry.
func TestInstall_UpgradeFailureNothingOutdated(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{}, nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.Contains(t, buf.String(), "nothing is left outdated")
}

// TestInstall_UpgradeFailureUnknownSubsetRecordsSentinel asserts that
// when `brew outdated` itself fails, the UpgradeAll sentinel is
// recorded so a retry re-runs the full upgrade.
func TestInstall_UpgradeFailureUnknownSubsetRecordsSentinel(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return(nil, errors.New("brew broken")).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedUpgrades, 1)
	assert.Equal(t, UpgradeAll, summary.FailedUpgrades[0].Name)
	assert.Contains(t, summary.FailedUpgrades[0].Reason, "brew broken")
}

// TestInstall_CaskHardErrorLimps asserts an InstallCask call that
// errors outright (not a StatusFailed outcome) is contained: the error
// text lands in FailedCasks as the Reason and the remaining casks are
// still attempted.
func TestInstall_CaskHardErrorLimps(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllFormulaeInstalls(fake)
	for _, c := range CommonCasks {
		if c == "raycast" {
			fake.On("InstallCask", mock.Anything, "raycast").
				Return(brew.CaskOutcome{}, errors.New("disk full")).Once()
			continue
		}
		fake.On("InstallCask", mock.Anything, c).
			Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	}
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Equal(t, "raycast", summary.FailedCasks[0].Name)
	assert.Equal(t, "disk full", summary.FailedCasks[0].Reason)
	// The .Once() expectation per remaining cask proves the loop kept
	// going past the hard error.
	fake.AssertExpectations(t)
}

// TestInstall_MissingBrewBinaryAborts asserts an error wrapping
// exec.ErrNotFound is catastrophic: the run aborts immediately instead
// of limping through every package against a missing binary.
func TestInstall_MissingBrewBinaryAborts(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).
		Return(fmt.Errorf("brew upgrade: %w", exec.ErrNotFound)).Once()

	var buf bytes.Buffer
	_, err := Install(context.Background(), &buf, fake, Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, exec.ErrNotFound)
	fake.AssertNotCalled(t, "Install", mock.Anything, mock.Anything)
	fake.AssertNotCalled(t, "InstallCask", mock.Anything, mock.Anything)
}

// TestInstall_InitialPassCatastrophicAborts asserts the formula-group
// and cask-loop triage on the initial pass also aborts on a missing
// brew binary, instead of recording it per package and prompting
// against a dead brew. The retry pass has its own twin table in
// retry_test.go — these call sites are separate code.
func TestInstall_InitialPassCatastrophicAborts(t *testing.T) {
	cases := []struct {
		name string
		wire func(fake *brewtest.FakeRunner)
	}{
		{
			name: "formula group",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
				fake.On("Install", mock.Anything, Formulae).
					Return(fmt.Errorf("brew install: %w", exec.ErrNotFound)).Once()
			},
		},
		{
			name: "formula isolation",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
				fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
				fake.On("Install", mock.Anything, []string{Formulae[0]}).
					Return(fmt.Errorf("brew install: %w", exec.ErrNotFound)).Once()
			},
		},
		{
			name: "cask loop",
			wire: func(fake *brewtest.FakeRunner) {
				expectAllFormulaeInstalls(fake)
				fake.On("InstallCask", mock.Anything, CommonCasks[0]).
					Return(brew.CaskOutcome{}, fmt.Errorf("brew install --cask: %w", exec.ErrNotFound)).Once()
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := brewtest.NewFakeRunner()
			tc.wire(fake)

			var buf bytes.Buffer
			_, err := Install(context.Background(), &buf, fake, Opts{})
			require.Error(t, err)
			require.ErrorIs(t, err, exec.ErrNotFound)
		})
	}
}

// TestInstall_MultipleFailuresAccumulate asserts the failure lists
// append across packages and groups — two failed casks plus failed
// formulae from two different groups must all survive into the
// summary (an append-to-overwrite regression would drop all but the
// last).
func TestInstall_MultipleFailuresAccumulate(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, DevTools).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{DevTools[0]}).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	for _, c := range CommonCasks {
		switch c {
		case "zoom", "raycast":
			fake.On("InstallCask", mock.Anything, c).
				Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
		default:
			fake.On("InstallCask", mock.Anything, c).
				Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
		}
	}
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedFormulae, 2)
	assert.Equal(t, Formulae[0], summary.FailedFormulae[0].Name)
	assert.Equal(t, DevTools[0], summary.FailedFormulae[1].Name)
	require.Len(t, summary.FailedCasks, 2)
	assert.ElementsMatch(t, []string{"zoom", "raycast"},
		[]string{summary.FailedCasks[0].Name, summary.FailedCasks[1].Name})
}

// TestInstall_UnrunnableBrewBinaryAborts asserts the broader
// catastrophic class: a brew that LookPath finds but exec cannot run
// (*exec.Error, e.g. clobbered permission bits) aborts the run just
// like a missing one.
func TestInstall_UnrunnableBrewBinaryAborts(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).
		Return(fmt.Errorf("brew upgrade: %w", &exec.Error{Name: "brew", Err: fs.ErrPermission})).Once()

	var buf bytes.Buffer
	_, err := Install(context.Background(), &buf, fake, Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrPermission)
	fake.AssertNotCalled(t, "Install", mock.Anything, mock.Anything)
}

// TestInstall_OutdatedMissingBrewBinaryAborts asserts a catastrophic
// `brew outdated` failure propagates instead of being folded into the
// UpgradeAll sentinel — only ordinary failures fall back to it.
func TestInstall_OutdatedMissingBrewBinaryAborts(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).
		Return(nil, fmt.Errorf("brew outdated: %w", exec.ErrNotFound)).Once()

	var buf bytes.Buffer
	_, err := Install(context.Background(), &buf, fake, Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, exec.ErrNotFound)
	fake.AssertNotCalled(t, "Install", mock.Anything, mock.Anything)
}

// TestInstall_FormulaGroupTransientFailureRecovers asserts a group
// failure followed by every isolated install succeeding leaves
// FailedFormulae empty — a transient group error alone is not a
// failure.
func TestInstall_FormulaGroupTransientFailureRecovers(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("transient")).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	expectAllCasksInstall(fake, CommonCasks)
	expectAllCasksInstall(fake, BetaCasks)

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.Contains(t, buf.String(), "every formula installed alone; continuing")
}

// TestInstall_ContextCancellation asserts that a pre-cancelled context
// surfaces as a context.Canceled error from Install (via Upgrade) —
// cancellation is catastrophic, not a limp-and-report failure.
func TestInstall_ContextCancellation(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(context.Canceled).Once()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	_, err := Install(ctx, &buf, fake, Opts{})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestCuratedListsContainTestCasks pins the cask names the mock
// fixtures across the test suites key on, so removing one from the
// curated lists fails here with a clear message instead of as a
// confusing mock-expectation mismatch elsewhere.
func TestCuratedListsContainTestCasks(t *testing.T) {
	for _, cask := range []string{"zoom", "docker", "raycast"} {
		assert.Contains(t, CommonCasks, cask)
	}
}

// TestInstall_PersonalCasksGated is a table test asserting PersonalCasks
// are present only when Opts.Personal is true.
func TestInstall_PersonalCasksGated(t *testing.T) {
	cases := []struct {
		name      string
		personal  bool
		wantCalls int
	}{
		{"personal=false", false, len(CommonCasks) + len(BetaCasks)},
		{"personal=true", true, len(CommonCasks) + len(BetaCasks) + len(PersonalCasks)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := brewtest.NewFakeRunner()
			expectAllFormulaeInstalls(fake)
			expectAllCasksInstall(fake, CommonCasks)
			expectAllCasksInstall(fake, BetaCasks)
			if tc.personal {
				expectAllCasksInstall(fake, PersonalCasks)
			}

			var buf bytes.Buffer
			_, err := Install(context.Background(), &buf, fake, Opts{Personal: tc.personal})
			require.NoError(t, err)
			// fake.AssertExpectations checks the exact wantCalls count
			// through the .Once() expectations registered above.
			fake.AssertExpectations(t)
		})
	}
}
