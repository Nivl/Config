// Package appsetuptest provides testify-mock-backed fakes for the
// appsetup package. It lives in a separate sub-package so testify
// only flows into test binaries — production builds that import
// appsetup never pull testify in.
package appsetuptest

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// FakeCmdRunner is a testify/mock-backed fake for appsetup.CmdRunner.
// Tests set expectations via fake.On("Run", mock.Anything, "ssh-keygen", ...).Return(nil)
// and fake.On("Capture", mock.Anything, "gh", ...).Return([]byte(...), nil).
type FakeCmdRunner struct {
	mock.Mock
}

// NewFakeCmdRunner returns a zero-valued FakeCmdRunner ready for
// expectation setup.
func NewFakeCmdRunner() *FakeCmdRunner { return &FakeCmdRunner{} }

// Run implements appsetup.CmdRunner via the embedded mock.
func (f *FakeCmdRunner) Run(ctx context.Context, name string, args ...string) error {
	callArgs := make([]any, 0, 2+len(args))
	callArgs = append(callArgs, ctx, name)
	for _, a := range args {
		callArgs = append(callArgs, a)
	}
	return f.Called(callArgs...).Error(0)
}

// Capture implements appsetup.CmdRunner via the embedded mock.
func (f *FakeCmdRunner) Capture(ctx context.Context, name string, args ...string) ([]byte, error) {
	callArgs := make([]any, 0, 2+len(args))
	callArgs = append(callArgs, ctx, name)
	for _, a := range args {
		callArgs = append(callArgs, a)
	}
	res := f.Called(callArgs...)
	v, _ := res.Get(0).([]byte)
	return v, res.Error(1)
}
