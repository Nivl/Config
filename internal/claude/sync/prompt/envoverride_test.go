package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnvOverride_KnownValues maps the three valid
// CLAUDE_MERGE_RESOLUTION values to their Choice + ok=true.
func TestEnvOverride_KnownValues(t *testing.T) {
	cases := []struct {
		env  string
		want Choice
	}{
		{"keep-local", ChoiceKeepLocal},
		{"take-remote", ChoiceTakeRemote},
		{"skip", ChoiceSkip},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			got, ok := parseMergeResolution(tc.env)
			assert.True(t, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestEnvOverride_OtherValuesFallThrough returns ok=false for empty
// strings and any non-canonical value — the parser matches only the
// three exact strings keep-local/take-remote/skip.
func TestEnvOverride_OtherValuesFallThrough(t *testing.T) {
	cases := []string{"", "Keep-Local", "KEEP-LOCAL", "true", "yes", "1", "garbage"}
	for _, env := range cases {
		t.Run("env="+env, func(t *testing.T) {
			_, ok := parseMergeResolution(env)
			assert.False(t, ok)
		})
	}
}
