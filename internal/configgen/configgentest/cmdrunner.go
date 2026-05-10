// Package configgentest provides testify-mock-backed fakes for the
// configgen package. It lives in a separate sub-package so testify
// only flows into test binaries — production builds that import
// configgen never pull testify in.
package configgentest

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// FakeCmdRunner is a testify/mock-based fake for configgen.CmdRunner.
// Tests set expectations with fake.On("Run", mock.Anything, "killall",
// "gpg-agent").Return(<error>).
type FakeCmdRunner struct {
	mock.Mock
}

// NewFakeCmdRunner returns a zero-valued fake ready for expectation
// setup.
func NewFakeCmdRunner() *FakeCmdRunner { return &FakeCmdRunner{} }

// Run implements configgen.CmdRunner via the embedded mock. The mock
// matches on (ctx, name, args[0], args[1], ...) — testify auto-flattens
// variadics when call sites pass them as separate args.
func (f *FakeCmdRunner) Run(ctx context.Context, name string, args ...string) error {
	callArgs := make([]any, 0, 2+len(args))
	callArgs = append(callArgs, ctx, name)
	for _, a := range args {
		callArgs = append(callArgs, a)
	}
	return f.Called(callArgs...).Error(0)
}
