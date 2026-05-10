package appsetuptest

import (
	"github.com/stretchr/testify/mock"
)

// FakeYesNoPrompter is a testify/mock-backed fake for appsetup.YesNoPrompter.
// Tests set expectations via fake.On("AskYesNo", "Setup Github").Return(true, nil).
type FakeYesNoPrompter struct {
	mock.Mock
}

// NewFakeYesNoPrompter returns a zero-valued fake ready for setup.
func NewFakeYesNoPrompter() *FakeYesNoPrompter { return &FakeYesNoPrompter{} }

// AskYesNo implements appsetup.YesNoPrompter via the embedded mock.
func (f *FakeYesNoPrompter) AskYesNo(prompt string) (bool, error) {
	args := f.Called(prompt)
	return args.Bool(0), args.Error(1)
}
