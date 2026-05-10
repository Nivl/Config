package appsetup

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYesNoPrompter_Yes verifies y/Y returns true.
func TestYesNoPrompter_Yes(t *testing.T) {
	p := NewYesNoPrompter(strings.NewReader("y\n"), &bytes.Buffer{})
	got, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.True(t, got)
}

// TestYesNoPrompter_YesCapital — capital Y also returns true (matches
// bash ${answer:0:1} dispatch).
func TestYesNoPrompter_YesCapital(t *testing.T) {
	p := NewYesNoPrompter(strings.NewReader("Yes\n"), &bytes.Buffer{})
	got, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.True(t, got)
}

// TestYesNoPrompter_No — n/N returns false.
func TestYesNoPrompter_No(t *testing.T) {
	p := NewYesNoPrompter(strings.NewReader("n\n"), &bytes.Buffer{})
	got, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.False(t, got)
}

// TestYesNoPrompter_InvalidLoops — non-y/n input retries until valid.
// Output should contain "Invalid value" once before the user types
// the valid answer.
func TestYesNoPrompter_InvalidLoops(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewYesNoPrompter(strings.NewReader("maybe\ny\n"), out)
	got, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.True(t, got)
	assert.Equal(t, 1, strings.Count(out.String(), "Invalid value"))
}

// TestYesNoPrompter_EmptyLineLoops — empty input (just \n) retries
// like any other invalid input.
func TestYesNoPrompter_EmptyLineLoops(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewYesNoPrompter(strings.NewReader("\nn\n"), out)
	got, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.False(t, got)
	assert.Equal(t, 1, strings.Count(out.String(), "Invalid value"))
}

// TestYesNoPrompter_EOFReturnsError — closed stdin returns errNoInput.
func TestYesNoPrompter_EOFReturnsError(t *testing.T) {
	p := NewYesNoPrompter(strings.NewReader(""), &bytes.Buffer{})
	_, err := p.AskYesNo("Setup Github")
	require.Error(t, err)
	assert.ErrorIs(t, err, errNoInput)
}

// TestYesNoPrompter_PromptStringContainsHint — the rendered prompt
// uses the "<prompt> (y/n)? " format.
func TestYesNoPrompter_PromptStringContainsHint(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewYesNoPrompter(strings.NewReader("y\n"), out)
	_, err := p.AskYesNo("Setup Github")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Setup Github (y/n)? ")
}
