// Package brewtest provides testify-mock-backed fakes for the brew
// package's interfaces. It lives in a separate sub-package so the
// testify dependency only flows into test binaries — production builds
// that import brew never pull testify in.
package brewtest

import (
	"context"

	"github.com/Nivl/config/internal/brew"

	"github.com/stretchr/testify/mock"
)

// FakeRunner is a testify-mock-backed brew.Runner. Set up expectations
// with fake.On("MethodName", args...).Return(...).
type FakeRunner struct {
	mock.Mock
}

// NewFakeRunner returns a fresh FakeRunner ready for expectation setup.
func NewFakeRunner() *FakeRunner { return &FakeRunner{} }

// Upgrade implements brew.Runner; delegates to the embedded mock.
// Variadic args are bundled into a single []string slot in the mock call.
func (f *FakeRunner) Upgrade(ctx context.Context, packages ...string) error {
	return f.Called(ctx, packages).Error(0)
}

// Outdated implements brew.Runner; delegates to the embedded mock.
// The first return is nil-safe: tests that register .Return(nil, err)
// for the error path get a typed-nil []string instead of a panic from
// a failed type assertion on an untyped nil.
func (f *FakeRunner) Outdated(ctx context.Context, _ ...string) ([]string, error) {
	args := f.Called(ctx)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// OutdatedCasks implements brew.Runner; delegates to the embedded mock.
// Nil-safe on the first return, same as Outdated.
func (f *FakeRunner) OutdatedCasks(ctx context.Context) ([]string, error) {
	args := f.Called(ctx)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// Install implements brew.Runner; delegates to the embedded mock.
// Variadic args are bundled into a single []string slot in the mock call.
func (f *FakeRunner) Install(ctx context.Context, formulae ...string) error {
	return f.Called(ctx, formulae).Error(0)
}

// InstallCask implements brew.Runner; delegates to the embedded mock.
func (f *FakeRunner) InstallCask(ctx context.Context, cask string) (brew.CaskOutcome, error) {
	args := f.Called(ctx, cask)
	v, _ := args.Get(0).(brew.CaskOutcome)
	return v, args.Error(1)
}

// IsCaskInstalled implements brew.Runner; delegates to the embedded mock.
func (f *FakeRunner) IsCaskInstalled(ctx context.Context, cask string) (bool, error) {
	args := f.Called(ctx, cask)
	return args.Bool(0), args.Error(1)
}

// IsAppRunning implements brew.Runner; delegates to the embedded mock.
func (f *FakeRunner) IsAppRunning(ctx context.Context, app string) (bool, error) {
	args := f.Called(ctx, app)
	return args.Bool(0), args.Error(1)
}

// ListCaskApps implements brew.Runner; delegates to the embedded mock.
// The first return is nil-safe: tests that register .Return(nil, err)
// for the error path get a typed-nil []string instead of a panic from
// a failed type assertion on an untyped nil.
func (f *FakeRunner) ListCaskApps(ctx context.Context, cask string) ([]string, error) {
	args := f.Called(ctx, cask)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}
