//go:build darwin

package brew

import (
	"context"
	"io"
	"testing"

	"github.com/Nivl/config/internal/iox"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_IsAppRunning_NotRunning asserts that a guaranteed-not-running
// fake app name returns false. We can't reliably assert TRUE without
// orchestrating a real app launch, so we only verify the negative case.
func TestRunner_IsAppRunning_NotRunning(t *testing.T) {
	r := NewRunner(iox.Streams{Out: io.Discard, Err: io.Discard})
	running, err := r.IsAppRunning(context.Background(), "ThisAppDefinitelyDoesNotExist_xyzzy")
	require.NoError(t, err)
	assert.False(t, running)
}

// TestEscapeAppleScriptString verifies the two characters that have
// special meaning in an AppleScript double-quoted string get escaped,
// with backslash done first so the double-quote escape's backslash
// isn't itself escaped.
func TestEscapeAppleScriptString(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`plain`, `plain`},
		{`with "quote"`, `with \"quote\"`},
		{`with \backslash`, `with \\backslash`},
		{`mixed "a\b"`, `mixed \"a\\b\"`},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, escapeAppleScriptString(c.in))
	}
}
