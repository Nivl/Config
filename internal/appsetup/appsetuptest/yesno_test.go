package appsetuptest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFakeYesNoPrompter_RoundTrip verifies the fake routes through
// testify's mock.
func TestFakeYesNoPrompter_RoundTrip(t *testing.T) {
	fake := NewFakeYesNoPrompter()
	fake.On("AskYesNo", "Setup Github").Return(true, nil)

	got, err := fake.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.True(t, got)
	fake.AssertExpectations(t)
}
