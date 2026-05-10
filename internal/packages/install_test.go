package packages

import (
	"bytes"
	"context"
	"errors"
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
	fake.On("Upgrade", mock.Anything).Return(nil).Once()
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
	assert.Empty(t, summary.Failed)
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
	assert.Empty(t, summary.Failed)
	fake.AssertExpectations(t)
}

// TestInstall_CaskFailureCapturesReason asserts that StatusFailed with a
// Reason populates summary.Failed verbatim.
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
	require.Len(t, summary.Failed, 1)
	assert.Equal(t, "raycast", summary.Failed[0].Name)
	assert.Equal(t, "Error: raycast download failed", summary.Failed[0].Reason)
	fake.AssertExpectations(t)
}

// TestInstall_FormulaFailureAbortsRun asserts that a formula install error
// returns a wrapped error AND prevents any cask install from running.
func TestInstall_FormulaFailureAbortsRun(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything).Return(nil).Once()
	fake.On("Install", mock.Anything, Formulae).Return(errors.New("network down")).Once()
	// No further calls expected — Install must abort here.

	var buf bytes.Buffer
	summary, err := Install(context.Background(), &buf, fake, Opts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network down")
	assert.Empty(t, summary.Skipped)
	assert.Empty(t, summary.Failed)
	fake.AssertExpectations(t)
}

// TestInstall_ContextCancellation asserts that a pre-cancelled context
// surfaces as a context.Canceled error from Install (via Upgrade).
func TestInstall_ContextCancellation(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything).Return(context.Canceled).Once()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	_, err := Install(ctx, &buf, fake, Opts{})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
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
