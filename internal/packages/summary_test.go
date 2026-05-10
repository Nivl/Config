package packages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSummary_Print_EmptyOmitsBothSections asserts that a Summary
// with no skipped or failed casks prints nothing.
func TestSummary_Print_EmptyOmitsBothSections(t *testing.T) {
	var buf bytes.Buffer
	(Summary{}).Print(&buf)
	assert.Empty(t, buf.String())
}

// TestSummary_Print_SkippedSection asserts the Skipped section format.
func TestSummary_Print_SkippedSection(t *testing.T) {
	var buf bytes.Buffer
	Summary{Skipped: []string{"docker", "warp"}}.Print(&buf)
	want := "\nSkipped cask updates because the app is running:\n" +
		"\t- docker\n" +
		"\t- warp\n"
	assert.Equal(t, want, buf.String())
}

// TestSummary_Print_FailedSection asserts the Failed section format.
func TestSummary_Print_FailedSection(t *testing.T) {
	var buf bytes.Buffer
	Summary{Failed: []FailedCask{
		{Name: "raycast", Reason: "Error: raycast download failed"},
	}}.Print(&buf)
	want := "\nFailed cask installs or upgrades:\n" +
		"\t- raycast: Error: raycast download failed\n"
	assert.Equal(t, want, buf.String())
}

// TestSummary_HasFailures asserts the boolean accessor.
func TestSummary_HasFailures(t *testing.T) {
	assert.False(t, Summary{}.HasFailures())
	assert.False(t, Summary{Skipped: []string{"x"}}.HasFailures())
	assert.True(t, Summary{Failed: []FailedCask{{Name: "x"}}}.HasFailures())
}
