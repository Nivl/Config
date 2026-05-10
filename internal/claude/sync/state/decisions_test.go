package state

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadDecisions_MissingFileReturnsEmpty verifies that a missing
// decisions.json yields an empty (but non-nil) Decisions struct, not an
// error. This is the first-run path before EnsureStateDir has seeded
// the file.
func TestLoadDecisions_MissingFileReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	d, err := LoadDecisions(p)
	require.NoError(t, err)
	assert.Equal(t, 0, d.Version) // zero value
	assert.NotNil(t, d.Settings)
	assert.Empty(t, d.Settings)
	assert.NotNil(t, d.Files)
	assert.Empty(t, d.Files)
}

// TestLoadDecisions_ParsesBashWrittenSchema verifies LoadDecisions
// can read a decisions.json with the standard schema, including
// compact-JSON settings keys.
func TestLoadDecisions_ParsesBashWrittenSchema(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	content := `{"version":1,"settings":{"[\"permissions\",\"allow\"]":"local","[\"model\"]":"remote"},"files":{"CLAUDE.md":"local"}}` + "\n"
	require.NoError(t, os.WriteFile(p.DecisionsFile, []byte(content), 0o644))

	d, err := LoadDecisions(p)
	require.NoError(t, err)
	assert.Equal(t, 1, d.Version)
	assert.Equal(t, ChoiceLocal, d.Settings[`["permissions","allow"]`])
	assert.Equal(t, ChoiceRemote, d.Settings[`["model"]`])
	assert.Equal(t, ChoiceLocal, d.Files["CLAUDE.md"])
}

// TestSaveDecisions_AtomicWrite verifies the file is written atomically
// and round-trips through LoadDecisions.
func TestSaveDecisions_AtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	p := NewPaths(tmp, tmp)
	require.NoError(t, os.MkdirAll(p.StateDir, 0o755))

	want := Decisions{
		Version: 1,
		Settings: map[string]Choice{
			`["model"]`: ChoiceLocal,
		},
		Files: map[string]Choice{},
	}
	require.NoError(t, SaveDecisions(p, want))

	got, err := LoadDecisions(p)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
