package packages

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSummary_PrintSkipped_EmptyPrintsNothing asserts that a Summary
// with nothing skipped prints nothing — failures never leak into the
// skipped trailer.
func TestSummary_PrintSkipped_EmptyPrintsNothing(t *testing.T) {
	var buf bytes.Buffer
	Summary{FailedCasks: []FailedItem{{Name: "raycast", Reason: "boom"}}}.PrintSkipped(&buf)
	assert.Empty(t, buf.String())
}

// TestSummary_PrintSkipped_Format asserts the skipped trailer format.
func TestSummary_PrintSkipped_Format(t *testing.T) {
	var buf bytes.Buffer
	Summary{Skipped: []string{"docker", "warp"}}.PrintSkipped(&buf)
	want := "\nSkipped cask updates because the app is running:\n" +
		"\t- docker\n" +
		"\t- warp\n"
	assert.Equal(t, want, buf.String())
}

// TestSummary_PrintFailures_Sections asserts each failure section's
// title and format, in upgrade → formula → cask order.
func TestSummary_PrintFailures_Sections(t *testing.T) {
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
	}.PrintFailures(&buf)
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
