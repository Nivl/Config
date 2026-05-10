// Package prompttest provides testify-mock-backed fakes for the prompt
// package's Prompter interface. It lives in a separate sub-package so
// the testify dependency only flows into test binaries — production
// builds that import prompt never pull testify in.
package prompttest

import (
	"context"

	"github.com/Nivl/config/internal/claude/sync/prompt"

	"github.com/stretchr/testify/mock"
)

// FakePrompter is a testify/mock-backed prompt.Prompter used in merge
// tests. Set up expectations with fake.On("Resolve", mock.Anything, req).Return(...).
type FakePrompter struct {
	mock.Mock
}

// NewFakePrompter returns a fresh FakePrompter.
func NewFakePrompter() *FakePrompter { return &FakePrompter{} }

// Resolve implements prompt.Prompter; delegates to the embedded mock.
func (f *FakePrompter) Resolve(ctx context.Context, req prompt.Request) (prompt.Choice, error) {
	args := f.Called(ctx, req)
	v, _ := args.Get(0).(prompt.Choice)
	return v, args.Error(1)
}

// Remember implements prompt.Prompter; delegates to the embedded mock.
func (f *FakePrompter) Remember(kind prompt.Kind, key string, c prompt.Choice) error {
	return f.Called(kind, key, c).Error(0)
}
