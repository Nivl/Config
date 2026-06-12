package packages

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Nivl/config/internal/brew"
	"github.com/Nivl/config/internal/brew/brewtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// expectOneCaskFailure wires every brew call to succeed except a
// single StatusFailed outcome for the named cask.
func expectOneCaskFailure(fake *brewtest.FakeRunner, cask string) {
	fake.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, cask).
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
	summary, err := InstallWithRetry(context.Background(), strings.NewReader(""), &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	assert.NotContains(t, out.String(), "What do you want to do?")
}

// TestInstallWithRetry_AbortReturnsErrAborted asserts choice 1 stops
// the run with the sentinel error and no retry calls.
func TestInstallWithRetry_AbortReturnsErrAborted(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake, "zoom")

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("1\n"), &out, fake, Opts{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAborted)
	require.Len(t, summary.FailedCasks, 1)
}

// TestInstallWithRetry_IgnoreContinues asserts choice 3 returns nil
// while the summary still carries the failures.
func TestInstallWithRetry_IgnoreContinues(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake, "zoom")

	var out bytes.Buffer
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("3\n"), &out, fake, Opts{})
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
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
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
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n3\n"), &out, fake, Opts{})
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
	summary, err := InstallWithRetry(context.Background(), strings.NewReader("2\n"), &out, fake, Opts{})
	require.NoError(t, err)
	assert.False(t, summary.HasFailures())
	fake.AssertExpectations(t)
}

// TestInstallWithRetry_EOFFailsLoudly asserts an unanswerable prompt
// (empty stdin) surfaces an error instead of looping or silently
// continuing — unattended runs must fail loudly.
func TestInstallWithRetry_EOFFailsLoudly(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectOneCaskFailure(fake, "zoom")

	var out bytes.Buffer
	_, err := InstallWithRetry(context.Background(), strings.NewReader(""), &out, fake, Opts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}
