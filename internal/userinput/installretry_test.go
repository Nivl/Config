package userinput

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstallRetryChoice_Dispatch covers the three valid answers,
// keyed off the first byte like the sibling prompts.
func TestInstallRetryChoice_Dispatch(t *testing.T) {
	cases := []struct {
		input string
		want  InstallRetryDecision
	}{
		{"1\n", InstallRetryAbort},
		{"2\n", InstallRetryAgain},
		{"3\n", InstallRetryIgnore},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var out bytes.Buffer
			got, err := InstallRetryChoice(strings.NewReader(tc.input), &out)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Contains(t, out.String(), "Choice (1/2/3)?")
		})
	}
}

// TestInstallRetryChoice_InvalidReprompts asserts garbage input prints
// the retry hint and loops until a valid answer arrives.
func TestInstallRetryChoice_InvalidReprompts(t *testing.T) {
	var out bytes.Buffer
	got, err := InstallRetryChoice(strings.NewReader("x\n2\n"), &out)
	require.NoError(t, err)
	assert.Equal(t, InstallRetryAgain, got)
	assert.Contains(t, out.String(), "Invalid value")
	assert.Equal(t, 2, strings.Count(out.String(), "Choice (1/2/3)?"))
}

// TestInstallRetryChoice_EOF asserts stdin EOF surfaces errNoInput so
// unattended runs fail loudly instead of looping forever.
func TestInstallRetryChoice_EOF(t *testing.T) {
	var out bytes.Buffer
	_, err := InstallRetryChoice(strings.NewReader(""), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}
