package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureStateDir_FreshCreatesAllArtifacts verifies the first-run
// behavior of EnsureStateDir: creates StateDir; seeds README.md with
// documentation; initializes decisions.json with
// {"version":1,"settings":{},"files":{}}\n.
func TestEnsureStateDir_FreshCreatesAllArtifacts(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))

	require.NoError(t, EnsureStateDir(p))

	assert.DirExists(t, p.StateDir)
	assert.FileExists(t, filepath.Join(p.StateDir, "README.md"))
	assert.FileExists(t, p.DecisionsFile)

	decisions, err := os.ReadFile(p.DecisionsFile)
	require.NoError(t, err)
	assert.Equal(t, `{"version":1,"settings":{},"files":{}}`+"\n", string(decisions)) //nolint:testifylint // byte-exact check on initial schema
}

// TestEnsureStateDir_IdempotentLeavesExistingFiles verifies that
// EnsureStateDir is idempotent: re-running it does not overwrite an
// existing decisions.json or README.md.
func TestEnsureStateDir_IdempotentLeavesExistingFiles(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.RepoDir, 0o755))
	require.NoError(t, EnsureStateDir(p))

	// Modify the files to non-default content.
	custom := `{"version":1,"settings":{"x":"local"},"files":{}}` + "\n"
	require.NoError(t, os.WriteFile(p.DecisionsFile, []byte(custom), 0o644))
	readmePath := filepath.Join(p.StateDir, "README.md")
	require.NoError(t, os.WriteFile(readmePath, []byte("custom"), 0o644))

	// Re-run; both should be untouched.
	require.NoError(t, EnsureStateDir(p))

	got, _ := os.ReadFile(p.DecisionsFile)
	assert.Equal(t, custom, string(got))
	gotReadme, _ := os.ReadFile(readmePath)
	assert.Equal(t, "custom", string(gotReadme))
}
