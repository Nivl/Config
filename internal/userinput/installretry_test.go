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
			got, err := InstallRetryChoice("", strings.NewReader(tc.input), &out)
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
	got, err := InstallRetryChoice("", strings.NewReader("x\n2\n"), &out)
	require.NoError(t, err)
	assert.Equal(t, InstallRetryAgain, got)
	assert.Contains(t, out.String(), "Invalid value")
	assert.Equal(t, 2, strings.Count(out.String(), "Choice (1/2/3)?"))
}

// TestInstallRetryChoice_EOF asserts stdin EOF surfaces errNoInput so
// unattended runs fail loudly instead of looping forever.
func TestInstallRetryChoice_EOF(t *testing.T) {
	var out bytes.Buffer
	_, err := InstallRetryChoice("", strings.NewReader(""), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no input")
}

// TestInstallRetryChoice_PreargSkipsPrompt asserts a pre-resolved
// abort|retry|ignore value bypasses the prompt entirely: nothing is
// written and stdin is never read.
func TestInstallRetryChoice_PreargSkipsPrompt(t *testing.T) {
	cases := []struct {
		prearg string
		want   InstallRetryDecision
	}{
		{"abort", InstallRetryAbort},
		{"retry", InstallRetryAgain},
		{"ignore", InstallRetryIgnore},
	}
	for _, tc := range cases {
		t.Run(tc.prearg, func(t *testing.T) {
			var out bytes.Buffer
			got, err := InstallRetryChoice(tc.prearg, strings.NewReader(""), &out)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Empty(t, out.String())
		})
	}
}

// TestInstallRetryChoice_PreargUnknownErrors asserts an unrecognized
// prearg value fails loud instead of silently falling back to the
// prompt — the cmd layer validates, so this means a wiring bug.
func TestInstallRetryChoice_PreargUnknownErrors(t *testing.T) {
	var out bytes.Buffer
	_, err := InstallRetryChoice("yolo", strings.NewReader(""), &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yolo")
}
