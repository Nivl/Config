package perms

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"gh pr view *", []string{"gh", "pr", "view"}},
		{"git symbolic-ref --short *", []string{"git", "symbolic-ref", "--short"}},
		{"rushx test *", []string{"rushx", "test"}},
		{"rush db:migrate:create:*", []string{"rush", "db:migrate:create"}},
		{"rush db*", []string{"rush", "db"}},
		{"git worktree list", []string{"git", "worktree", "list"}},
		{"/opt/homebrew/bin/gh pr view *", []string{"gh", "pr", "view"}},
		{"DD_SERVICE=app-api rush db:migrate:up:*", []string{"rush", "db:migrate:up"}},
		{"* --version", []string{"*", "--version"}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, toPrefix(c.in), "toPrefix(%q)", c.in)
	}
}

func TestUnwrapBash(t *testing.T) {
	in, ok := unwrapBash("Bash(gh pr view *)")
	assert.True(t, ok)
	assert.Equal(t, "gh pr view *", in)

	_, ok = unwrapBash("Read(//tmp/**)")
	assert.False(t, ok)
}

func TestDerive(t *testing.T) {
	allow := []string{
		"Bash(gh pr view *)",
		"Bash(/opt/homebrew/bin/gh pr view *)", // collapses with the line above
		"Bash(git symbolic-ref --short *)",
		"Bash(git add *)",
		"Bash(go build *)", // sandboxed, must be dropped
		"Bash(rushx test *)",
		"Read(//tmp/**)", // non-Bash, ignored
	}
	excluded := []string{"git *", "gh *", "go get *", "go mod *", "rushx test *"}

	trusted, exclPrefixes := Derive(allow, excluded)

	// Both lists come back sorted by Derive (NUL-joined key order).
	assert.Equal(t, [][]string{
		{"gh"}, {"git"}, {"go", "get"}, {"go", "mod"}, {"rushx", "test"},
	}, exclPrefixes)
	assert.Equal(t, [][]string{
		{"gh", "pr", "view"},
		{"git", "add"},
		{"git", "symbolic-ref", "--short"},
		{"rushx", "test"},
	}, trusted)
}

func TestWriteBashAllow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bash-allow-trusted.json")
	err := WriteBashAllow(
		path,
		[][]string{{"gh", "pr", "view"}},
		[][]string{{"git"}, {"gh"}},
	)
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	//nolint:testifylint // byte-exact format check, not semantic JSON
	assert.Equal(t, `{
  "excluded": [["git"], ["gh"]],
  "trusted": [["gh", "pr", "view"]]
}
`, string(got))
}

func TestWriteBashAllow_WrapsWhenInlineWouldExceedBudget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bash-allow-trusted.json")
	long := [][]string{
		{"gh", "pr", "view"},
		{"gh", "pr", "list"},
		{"gh", "pr", "diff"},
		{"gh", "pr", "checks"},
		{"gh", "release", "verify-asset"},
		{"gh", "release", "view"},
		{"gh", "release", "list"},
	}
	require.NoError(t, WriteBashAllow(path, long, [][]string{{"gh"}}))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	//nolint:testifylint // byte-exact format check, not semantic JSON
	assert.Equal(t, `{
  "excluded": [["gh"]],
  "trusted": [
    ["gh", "pr", "view"],
    ["gh", "pr", "list"],
    ["gh", "pr", "diff"],
    ["gh", "pr", "checks"],
    ["gh", "release", "verify-asset"],
    ["gh", "release", "view"],
    ["gh", "release", "list"]
  ]
}
`, string(got))
}

func TestWriteBashAllow_EmptyListsRenderAsArrays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	require.NoError(t, WriteBashAllow(path, nil, nil))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	//nolint:testifylint // byte-exact format check, not semantic JSON
	assert.Equal(t, "{\n  \"excluded\": [],\n  \"trusted\": []\n}\n", string(got))
}
