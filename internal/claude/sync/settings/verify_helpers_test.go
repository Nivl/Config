package settings

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Nivl/config/internal/claude/sync/state"
)

// TestValidJSON_InvalidRejectsBraces verifies that obviously malformed
// JSON (unquoted keys) is rejected — the gate that triggers Merge's
// soft-outcome warnings.
func TestValidJSON_InvalidRejectsBraces(t *testing.T) {
	assert.False(t, validJSON([]byte("{not json}")))
}

// TestValidJSON_EmptyObjectAccepted verifies the canonical empty object
// is treated as valid JSON.
func TestValidJSON_EmptyObjectAccepted(t *testing.T) {
	assert.True(t, validJSON([]byte("{}")))
}

// TestKeyForSettings_CompactArray verifies the byte-exact compact-
// JSON encoding of a path array used as a decisions.json key. The
// key format is byte-stable across versions.
func TestKeyForSettings_CompactArray(t *testing.T) {
	result := keyForSettings([]string{"permissions", "allow"})
	assert.Equal(t, `["permissions","allow"]`, result)
}

// newHelperTestPaths builds a Paths with distinct config and home dirs
// (avoids the NewPaths(tmp, tmp) aliasing footgun where RepoDir and
// HomeDir collide).
func newHelperTestPaths(t *testing.T) state.Paths {
	t.Helper()
	tmp := t.TempDir()
	configDir := tmp + "/config"
	homeDir := tmp + "/home"
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	p := state.NewPaths(configDir, homeDir)
	require.NoError(t, state.EnsureStateDir(p))
	return p
}

// TestBaseShortSHA_NoAnchor verifies the "none" sentinel returned
// when last-sync-commit is absent (used as the placeholder in the
// conflict prompt header).
func TestBaseShortSHA_NoAnchor(t *testing.T) {
	p := newHelperTestPaths(t)
	assert.Equal(t, "none", baseShortSHA(p))
}

// TestBaseShortSHA_LongSHA verifies the standard 7-char short rendering
// of a full SHA.
func TestBaseShortSHA_LongSHA(t *testing.T) {
	p := newHelperTestPaths(t)
	require.NoError(t, os.WriteFile(p.LastSyncFile, []byte("abcdef1234567890\n"), 0o644))
	assert.Equal(t, "abcdef1", baseShortSHA(p))
}

// TestBaseShortSHA_ShortSHA verifies that a SHA shorter than 7 chars is
// returned verbatim (edge case for synthetic test SHAs).
func TestBaseShortSHA_ShortSHA(t *testing.T) {
	p := newHelperTestPaths(t)
	require.NoError(t, os.WriteFile(p.LastSyncFile, []byte("abc\n"), 0o644))
	assert.Equal(t, "abc", baseShortSHA(p))
}
