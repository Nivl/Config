package packages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSummary_Print_EmptyOmitsAllSections asserts that a Summary with
// nothing skipped and nothing failed prints nothing.
func TestSummary_Print_EmptyOmitsAllSections(t *testing.T) {
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

// TestSummary_Print_FailureSections asserts each failure section's
// title and format, in upgrade → formula → cask order.
func TestSummary_Print_FailureSections(t *testing.T) {
	var buf bytes.Buffer
	Summary{
		FailedUpgrades: []FailedItem{
			{Name: "the-unarchiver", Reason: "still outdated after brew upgrade (see brew output above)"},
		},
		FailedFormulae: []FailedItem{
			{Name: "go", Reason: "network down"},
		},
		FailedCasks: []FailedItem{
			{Name: "raycast", Reason: "Error: raycast download failed"},
		},
	}.Print(&buf)
	want := "\nFailed upgrades:\n" +
		"\t- the-unarchiver: still outdated after brew upgrade (see brew output above)\n" +
		"\nFailed formula installs:\n" +
		"\t- go: network down\n" +
		"\nFailed cask installs or upgrades:\n" +
		"\t- raycast: Error: raycast download failed\n"
	assert.Equal(t, want, buf.String())
}

// TestSummary_PrintFailures_OmitsSkipped asserts the prompt-loop
// variant repeats only the failures, never the skipped trailer.
func TestSummary_PrintFailures_OmitsSkipped(t *testing.T) {
	var buf bytes.Buffer
	Summary{
		Skipped:     []string{"docker"},
		FailedCasks: []FailedItem{{Name: "raycast", Reason: "boom"}},
	}.PrintFailures(&buf)
	assert.NotContains(t, buf.String(), "Skipped cask updates")
	assert.Contains(t, buf.String(), "raycast: boom")
}

// TestSummary_HasFailures asserts the boolean accessor covers every
// failure kind and ignores skips.
func TestSummary_HasFailures(t *testing.T) {
	assert.False(t, Summary{}.HasFailures())
	assert.False(t, Summary{Skipped: []string{"x"}}.HasFailures())
	assert.True(t, Summary{FailedUpgrades: []FailedItem{{Name: "x", Reason: ""}}}.HasFailures())
	assert.True(t, Summary{FailedFormulae: []FailedItem{{Name: "x", Reason: ""}}}.HasFailures())
	assert.True(t, Summary{FailedCasks: []FailedItem{{Name: "x", Reason: ""}}}.HasFailures())
}
