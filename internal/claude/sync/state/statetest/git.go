// Package statetest provides testify-mock-backed fakes for the state
// package's Git interface. It lives in a separate sub-package so the
// testify dependency only flows into test binaries — production builds
// that import state never pull testify in.
package statetest

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// FakeGit is a testify/mock-backed state.Git used in tests. Set up
// expectations with fake.On("ShowBase", mock.Anything, "settings.json").Return(...).
type FakeGit struct {
	mock.Mock
}

// NewFakeGit returns a fresh FakeGit ready for expectation setup.
func NewFakeGit() *FakeGit { return &FakeGit{} }

// ShowBase implements state.Git; delegates to the embedded mock.
func (f *FakeGit) ShowBase(ctx context.Context, rel string) ([]byte, error) {
	args := f.Called(ctx, rel)
	v, _ := args.Get(0).([]byte)
	return v, args.Error(1)
}

// BaseHas implements state.Git; delegates to the embedded mock.
func (f *FakeGit) BaseHas(ctx context.Context, rel string) (bool, error) {
	args := f.Called(ctx, rel)
	return args.Bool(0), args.Error(1)
}

// ListTree implements state.Git; delegates to the embedded mock.
func (f *FakeGit) ListTree(ctx context.Context, dir string) ([]string, error) {
	args := f.Called(ctx, dir)
	v, _ := args.Get(0).([]string)
	return v, args.Error(1)
}

// HeadSHA implements state.Git; delegates to the embedded mock.
func (f *FakeGit) HeadSHA(ctx context.Context) (string, error) {
	args := f.Called(ctx)
	return args.String(0), args.Error(1)
}
