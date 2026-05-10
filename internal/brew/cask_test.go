package brew

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCaskApps verifies the brew info JSON parser extracts .app
// artifact names with the path prefix stripped and .app suffix
// removed.
func TestParseCaskApps(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []string
	}{
		{
			name: "single app",
			json: `{"casks":[{"artifacts":[{"app":["Docker.app"]}]}]}`,
			want: []string{"Docker"},
		},
		{
			name: "app with path prefix",
			json: `{"casks":[{"artifacts":[{"app":["Applications/Foo.app"]}]}]}`,
			want: []string{"Foo"},
		},
		{
			name: "multiple apps in artifact",
			json: `{"casks":[{"artifacts":[{"app":["A.app","B.app"]}]}]}`,
			want: []string{"A", "B"},
		},
		{
			name: "mixed artifacts — only .app survives",
			json: `{"casks":[{"artifacts":[{"binary":["foo"]},{"app":["Bar.app"]}]}]}`,
			want: []string{"Bar"},
		},
		{
			name: "no casks",
			json: `{"casks":[]}`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCaskApps([]byte(tc.json))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestExtractCaskFailureReason verifies that the helper returns the
// trimmed last non-empty line of stderr, with a fixed default for
// empty input.
func TestExtractCaskFailureReason(t *testing.T) {
	cases := []struct {
		name, stderr, want string
	}{
		{"trailing newlines", "Error: raycast download failed\n\n", "Error: raycast download failed"},
		{"middle blank lines", "junk\n\nError: real reason\n", "Error: real reason"},
		{"empty stderr", "", "Homebrew cask command failed without an error message"},
		{"only whitespace", "  \n\t\n  ", "Homebrew cask command failed without an error message"},
		{"single line, no newline", "boom", "boom"},
		{"leading/trailing space on last line", "first\n   real reason   \n", "real reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, extractCaskFailureReason([]byte(tc.stderr)))
		})
	}
}
