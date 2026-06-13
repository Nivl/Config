package packages

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/brew/brewtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// expectOneCaskFailure wires every brew call to succeed except a
// single StatusFailed outcome for the zoom cask.
func expectOneCaskFailure(fake *brewtest.FakeRunner) {
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)
}

// TestInstallWithRetry_NoFailuresNoPrompt asserts a clean run never
// touches stdin.
func TestInstallWithRetry_NoFailuresNoPrompt(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader(""), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.NotContains(t, out.String(), "What do you want to do?")
}

// TestInstallWithRetry_AbortReturnsErrAborted asserts choice 1 stops
// the run with the sentinel error and no retry calls.
func TestInstallWithRetry_AbortReturnsErrAborted(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("1\n"), &out, &out, fake, Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAborted)
	require.Len(t, summary.FailedCasks, 1)
}

// TestInstallWithRetry_IgnoreContinues asserts choice 3 returns nil
// while the summary still carries the failures.
func TestInstallWithRetry_IgnoreContinues(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("3\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Contains(t, out.String(), "Ignoring the failed packages and continuing.")
}

// TestInstallWithRetry_RetryClearsFailures asserts choice 2 re-attempts
// only the failed cask and a clean retry ends the loop.
func TestInstallWithRetry_RetryClearsFailures(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	// Initial pass touches every cask, the retry re-attempts only the
	// failed one — the exact count pins that nothing else was retried
	// (the catch-all expectation would absorb extra calls silently).
	fake.AssertNumberOfCalls(t, "InstallCask", len(CommonCasks)+len(BetaCasks)+1)
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_RetryThenIgnore asserts a still-failing retry
// re-prompts, and the buffered reader survives across both asks (the
// second answer is read from the same stdin).
func TestInstallWithRetry_RetryThenIgnore(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Twice()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n3\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Equal(t, 2, strings.Count(out.String(), "Choice (1/2/3)?"))
}

// TestInstallWithRetry_UpgradeRetryIsScoped asserts a retried upgrade
// targets only the packages that failed, not the whole system.
func TestInstallWithRetry_UpgradeRetryIsScoped(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"the-unarchiver"}, nil).Once()
	fake.On("Upgrade", mock.Anything, []string{"the-unarchiver"}).Return(nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_EOFFailsLoudly asserts an unanswerable prompt
// (empty stdin) surfaces an error instead of looping or silently
// continuing — unattended runs must fail loudly.
func TestInstallWithRetry_EOFFailsLoudly(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake)

	var out bytes.Buffer
	_, err := InstallWithRetry(context.Background(), strings.NewReader(""), &out, &out, fake, Opts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

// TestInstallWithRetry_SentinelRetryRerunsFullUpgrade asserts a retry
// consuming the UpgradeAll sentinel re-runs the unscoped upgrade — the
// sentinel name itself must never reach brew as a package name.
func TestInstallWithRetry_SentinelRetryRerunsFullUpgrade(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return(nil, errors.New("brew broken")).Once()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_ScopedRetryFailureFiltersToScope asserts that
// when a scoped retry fails again, only in-scope names land back in
// FailedUpgrades — packages that became outdated mid-run were never
// attempted by the retry and must not be reported as its failures.
func TestInstallWithRetry_ScopedRetryFailureFiltersToScope(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
	fake.On("Upgrade", mock.Anything, []string{"a"}).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"a", "b"}, nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n3\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedUpgrades, 1)
	assert.Equal(t, "a", summary.FailedUpgrades[0].Name)
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_FormulaRetryReattemptsOnlyFailed asserts the
// retry pass re-installs just the formula that failed in isolation,
// not its whole group.
func TestInstallWithRetry_FormulaRetryReattemptsOnlyFailed(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_RetryCarriesSkippedOver asserts skipped casks
// survive a retry pass: they are neither dropped nor re-attempted.
func TestInstallWithRetry_RetryCarriesSkippedOver(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "docker").
		Return(brew.CaskOutcome{Status: brew.StatusSkipped}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.Equal(t, []string{"docker"}, summary.Skipped)
	// Exactly one extra call beyond the initial pass: the failed cask.
	// The skipped one must not be re-attempted.
	fake.AssertNumberOfCalls(t, "InstallCask", len(CommonCasks)+len(BetaCasks)+1)
}

// TestInstallWithRetry_CaskHardErrorOnRetryRecorded asserts an
// InstallCask call that errors outright during the retry pass is
// contained the same way as in the initial pass.
func TestInstallWithRetry_CaskHardErrorOnRetryRecorded(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{}, errors.New("disk full")).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n3\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Equal(t, "disk full", summary.FailedCasks[0].Reason)
}

// TestInstallWithRetry_RetryCatastrophicAborts asserts each branch of
// the retry pass propagates catastrophic errors (here a missing brew
// binary) instead of recording them as ordinary failures and looping
// back to a prompt against a dead brew.
func TestInstallWithRetry_RetryCatastrophicAborts(t *testing.T) {
	cases := []struct {
		name string
		wire func(fake *brewtest.FakeRunner)
	}{
		{
			name: "upgrade",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
				fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
				fake.On("Upgrade", mock.Anything, []string{"a"}).
					Return(fmt.Errorf("brew upgrade: %w", exec.ErrNotFound)).Once()
				fake.On("Install", mock.Anything, mock.Anything).Return(nil)
			},
		},
		{
			name: "formula",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
				fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
				fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(errors.New("network down")).Once()
				fake.On("Install", mock.Anything, []string{Formulae[0]}).
					Return(fmt.Errorf("brew install: %w", exec.ErrNotFound)).Once()
				fake.On("Install", mock.Anything, mock.Anything).Return(nil)
			},
		},
		{
			name: "cask",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
				fake.On("Install", mock.Anything, mock.Anything).Return(nil)
				fake.On("InstallCask", mock.Anything, "zoom").
					Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
				fake.On("InstallCask", mock.Anything, "zoom").
					Return(brew.CaskOutcome{}, fmt.Errorf("brew install --cask: %w", exec.ErrNotFound)).Once()
			},
		},
		{
			name: "outdated re-check",
			wire: func(fake *brewtest.FakeRunner) {
				fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
				fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
				fake.On("Upgrade", mock.Anything, []string{"a"}).Return(errors.New("exit status 1")).Once()
				fake.On("Outdated", mock.Anything).
					Return(nil, fmt.Errorf("brew outdated: %w", exec.ErrNotFound)).Once()
				fake.On("Install", mock.Anything, mock.Anything).Return(nil)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := brewtest.NewFakeRunner()
			tc.wire(fake)
			fake.On("InstallCask", mock.Anything, mock.Anything).
				Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

			var out bytes.Buffer
			_, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
			require.Error(t, err)
			require.ErrorIs(t, err, exec.ErrNotFound)
		})
	}
}

// TestInstallWithRetry_RetryClearsAllFailureKinds asserts one retry
// pass walks all three failure lists: upgrades, formulae, and casks
// seeded together are all re-attempted and cleared in a single pass.
// Every retry attempt is a .Once() expectation, so a regression that
// skips one of the three blocks fails AssertExpectations.
func TestInstallWithRetry_RetryClearsAllFailureKinds(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
	fake.On("Upgrade", mock.Anything, []string{"a"}).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(errors.New("network down")).Once()
	fake.On("Install", mock.Anything, []string{Formulae[0]}).Return(nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_ScopedRetryOutdatedFailureKeepsScope asserts
// that when the post-retry outdated re-check fails ordinarily, the
// known scope is recorded as the failed set — the UpgradeAll sentinel
// (and its full-upgrade retry) is reserved for the genuinely unknown
// first-pass case.
func TestInstallWithRetry_ScopedRetryOutdatedFailureKeepsScope(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
	fake.On("Upgrade", mock.Anything, []string{"a"}).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return(nil, errors.New("brew flake")).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n3\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	require.Len(t, summary.FailedUpgrades, 1)
	assert.Equal(t, "a", summary.FailedUpgrades[0].Name)
	assert.Contains(t, summary.FailedUpgrades[0].Reason, "brew flake")
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_ScopedRetryFilterCanEmpty asserts that when a
// scoped retry fails but only out-of-scope names remain outdated, the
// filter empties the failed set and the loop ends clean — mid-run
// newcomers cannot keep the retry loop alive.
func TestInstallWithRetry_ScopedRetryFilterCanEmpty(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, []string(nil)).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"a"}, nil).Once()
	fake.On("Upgrade", mock.Anything, []string{"a"}).Return(errors.New("exit status 1")).Once()
	fake.On("Outdated", mock.Anything).Return([]string{"b"}, nil).Once()
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.Equal(t, 1, strings.Count(out.String(), "Choice (1/2/3)?"))
	assert.Contains(t, out.String(), "none of the retried packages is still outdated")
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_PreresolvedChoicesSkipPrompt asserts
// Opts.FailureResolution answers the ask without touching stdin:
// "abort" surfaces ErrAborted and "ignore" continues with the
// failures, neither ever printing the menu.
func TestInstallWithRetry_PreresolvedChoicesSkipPrompt(t *testing.T) {
	cases := []struct {
		resolution string
		wantErr    error
	}{
		{"abort", ErrAborted},
		{"ignore", nil},
	}
	for _, tc := range cases {
		t.Run(tc.resolution, func(t *testing.T) {
			fake := brewtest.NewFakeRunner()
			expectOneCaskFailure(fake)

			var out bytes.Buffer
			summary, err := InstallWithRetry(context.Background(), strings.NewReader(""), &out, &out, fake,
				Opts{FailureResolution: tc.resolution})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, summary.FailedCasks, 1)
			assert.NotContains(t, out.String(), "Choice (1/2/3)?")
		})
	}
}

// TestInstallWithRetry_PreresolvedRetryAppliesOnce asserts a
// pre-resolved "retry" answers only the first ask — when the retry
// leaves failures behind, the next decision is prompted so a package
// that never recovers cannot loop forever.
func TestInstallWithRetry_PreresolvedRetryAppliesOnce(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Twice()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("3\n"), &out, &out, fake,
		Opts{FailureResolution: "retry"})
	require.NoError(t, err)
	require.Len(t, summary.FailedCasks, 1)
	assert.Equal(t, 1, strings.Count(out.String(), "Choice (1/2/3)?"))
}
