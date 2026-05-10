package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChoice_String — all four branches: each documented Choice
// renders to its stable lowercase form, and an out-of-range value
// renders as "unknown" rather than the zero-value default. The
// strings are user-visible in "(env)" / "(remembered)" log lines,
// so a regression would shift the on-screen output.
func TestChoice_String(t *testing.T) {
	cases := []struct {
		choice Choice
		want   string
	}{
		{ChoiceKeepLocal, "keep-local"},
		{ChoiceTakeRemote, "take-remote"},
		{ChoiceSkip, "skip"},
		{Choice(99), "unknown"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.choice.String(), "Choice(%d)", int(tc.choice))
	}
}
