package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
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
// stdout buffer is wired into cfg.streams.Out so test assertions can
// inspect what installPackagesCmd writes.
func makeTestCfg(fake *brewtest.FakeRunner, stdout io.Writer) *appConfig {
	return &appConfig{
		cwd:           "",
		configDir:     "",
		streams:       iox.Streams{Out: stdout, Err: io.Discard},
		newBrewRunner: func(iox.Streams) brew.Runner { return fake },
		reporter:      dryrun.NewNullReporter(),
	}
}

// expectAllInstallsSucceed wires up every brew call to succeed cleanly.
func expectAllInstallsSucceed(fake *brewtest.FakeRunner) {
	fake.On("Upgrade", mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)
}

// TestInstallPackagesCmd_Success asserts a clean run returns no error
// and AssertExpectations is satisfied on the fake.
func TestInstallPackagesCmd_Success(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	expectAllInstallsSucceed(fake)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, &stdout)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false})
	require.NoError(t, err)
	fake.AssertExpectations(t)
}

// TestInstallPackagesCmd_FailuresExitNonZero asserts that any Failed
// cask causes installPackagesCmd to return a non-nil error.
func TestInstallPackagesCmd_FailuresExitNonZero(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	// First InstallCask call fails; rest succeed.
	fake.On("InstallCask", mock.Anything, "zoom").
		Return(brew.CaskOutcome{Status: brew.StatusFailed, Reason: "synthetic"}, nil).Once()
	fake.On("InstallCask", mock.Anything, mock.Anything).
		Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, &stdout)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some casks failed")
	assert.Contains(t, stdout.String(), "Failed cask installs or upgrades:")
	assert.Contains(t, stdout.String(), "zoom: synthetic")
}

// TestInstallPackagesCmd_PersonalParamIncludesPersonalCasks asserts
// that installPackagesParams{personal: true} flips the personal-cask
// gate. The flag/env resolution happens in newInstallPackagesCmd.RunE
// (covered by the resolve* tests in root_test.go); here we exercise
// the body directly with the already-resolved bool.
func TestInstallPackagesCmd_PersonalParamIncludesPersonalCasks(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything).Return(nil)
	// Two Install calls expected (formulae + fonts); collapsed by mock.Anything.
	fake.On("Install", mock.Anything, mock.Anything).Return(nil)
	// Any cask install (Common, Beta, AND Personal) succeeds.
	var personalCalled bool
	fake.On("InstallCask", mock.Anything, mock.MatchedBy(func(c string) bool {
		if slices.Contains(packages.PersonalCasks, c) {
			personalCalled = true
		}
		return true
	})).Return(brew.CaskOutcome{Status: brew.StatusInstalled}, nil)

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, &stdout)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: true})
	require.NoError(t, err)
	assert.True(t, personalCalled, "personal=true should have included PersonalCasks")
}

// TestInstallPackagesCmd_FormulaErrorPropagated asserts that a formula
// install failure surfaces as the returned error.
func TestInstallPackagesCmd_FormulaErrorPropagated(t *testing.T) {
	fake := brewtest.NewFakeRunner()
	fake.On("Upgrade", mock.Anything).Return(nil)
	fake.On("Install", mock.Anything, packages.Formulae).
		Return(errors.New("network down")).Once()

	var stdout bytes.Buffer
	cfg := makeTestCfg(fake, &stdout)
	err := installPackagesCmd(context.Background(), cfg, installPackagesParams{personal: false})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network down")
}
