package perms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_MissingFile — Load on a non-existent path returns an empty
// Settings (all three lists present and empty) rather than an error,
// so a fresh clone can `claude perms allow add` without manual setup.
func TestLoad_MissingFile(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	assert.Empty(t, s.List(ListAllow))
	assert.Empty(t, s.List(ListAsk))
	assert.Empty(t, s.List(ListDeny))
}

// TestLoad_EmptyFile — an empty (or whitespace-only) file is treated
// the same as a missing file: bootstrap empty lists.
func TestLoad_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, []byte("   \n"), 0o644))
	s, err := Load(path)
	require.NoError(t, err)
	assert.Empty(t, s.List(ListAllow))
}

// TestLoad_PreservesUnknownTopLevelKeys — Load+Save round-trips
// `model`, `theme`, and other top-level keys without modification.
// We're not the only writer to settings.json.
func TestLoad_PreservesUnknownTopLevelKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	initial := `{
  "model": "claude-opus-4-7",
  "permissions": {
    "allow": ["Bash(ls)"],
    "ask": [],
    "deny": []
  },
  "theme": "dark"
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	s, err := Load(path)
	require.NoError(t, err)
	require.NoError(t, s.Save(path))

	roundTripped, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(roundTripped, &got))
	assert.Equal(t, "claude-opus-4-7", got["model"])
	assert.Equal(t, "dark", got["theme"])
}

// TestLoad_PreservesUnknownPermissionsKeys — keys inside `permissions`
// that aren't allow/ask/deny (like `defaultMode`) survive round-trip.
func TestLoad_PreservesUnknownPermissionsKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	initial := `{
  "permissions": {
    "allow": [],
    "ask": [],
    "defaultMode": "ask",
    "deny": []
  }
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	s, err := Load(path)
	require.NoError(t, err)
	s.SetList(ListAllow, []string{"Bash(ls)"})
	require.NoError(t, s.Save(path))

	roundTripped, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]map[string]any
	require.NoError(t, json.Unmarshal(roundTripped, &got))
	assert.Equal(t, "ask", got["permissions"]["defaultMode"])
}

// TestSave_SortsListsAlphabetically — Save reorders each list
// alphabetically regardless of input order, matching the convention
// established by commit 2039803 ("claude: sort arrays alphabetically").
func TestSave_SortsListsAlphabetically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	require.NoError(t, err)
	s.SetList(ListAllow, []string{"Bash(zap)", "Bash(aardvark)", "Bash(mid)"})
	require.NoError(t, s.Save(path))

	roundTripped, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(aardvark)", "Bash(mid)", "Bash(zap)"},
		roundTripped.List(ListAllow))
}

// TestSave_DeduplicatesLists — Save strips exact-string duplicates so
// a caller can blindly append and rely on Save to clean up.
func TestSave_DeduplicatesLists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	require.NoError(t, err)
	s.SetList(ListAllow, []string{"Bash(ls)", "Bash(ls)", "Bash(cat)"})
	require.NoError(t, s.Save(path))

	roundTripped, _ := Load(path)
	assert.Equal(t, []string{"Bash(cat)", "Bash(ls)"}, roundTripped.List(ListAllow))
}

// TestSave_EmptyListIsBracketsNotNull — empty lists serialize as `[]`
// so the file stays human-friendly. A `null` value would break the
// schema other Claude tools depend on.
func TestSave_EmptyListIsBracketsNotNull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	require.NoError(t, err)
	require.NoError(t, s.Save(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"allow": []`)
	assert.NotContains(t, string(data), "null")
}

// TestSave_NormalizesNestedMultiLineRawValues — a `permsOther` value
// that holds multi-line JSON (e.g. an array or object inside
// `permissions`) round-trips with consistent 2-space indentation.
// Without the final json.Indent normalization, the inlined raw
// bytes would keep their original layout and produce jagged output.
func TestSave_NormalizesNestedMultiLineRawValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Pre-seed a multi-line permsOther value (additionalDirectories
	// is hypothetical — we just need any future-shaped non-scalar
	// inside permissions).
	initial := `{
  "permissions": {
    "additionalDirectories": [
      "/extra/dir"
    ],
    "allow": [],
    "ask": [],
    "deny": []
  }
}
`
	require.NoError(t, os.WriteFile(path, []byte(initial), 0o644))

	s, err := Load(path)
	require.NoError(t, err)
	s.SetList(ListAllow, []string{"Bash(ls)"})
	require.NoError(t, s.Save(path))

	roundTripped, err := os.ReadFile(path)
	require.NoError(t, err)
	str := string(roundTripped)
	assert.Contains(t, str, `"additionalDirectories": [`,
		"unknown nested keys must survive round-trip")
	// The inner array element must sit at 6-space indent (matches
	// the surrounding allow/ask/deny lists), not whatever indent
	// json.RawMessage happened to capture.
	assert.Contains(t, str, "      \"/extra/dir\"",
		"nested multi-line values must be re-indented consistently")
}

func TestSettings_ExcludedCommandsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "model": "opus",
  "permissions": { "allow": [], "ask": [], "deny": [] },
  "sandbox": {
    "enabled": true,
    "excludedCommands": ["git *", "gh *"],
    "filesystem": { "denyRead": ["/x"] }
  }
}
`), 0o644))

	s, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"git *", "gh *"}, s.ExcludedCommands())

	s.SetExcludedCommands([]string{"gh *", "docker *", "git *"})
	require.NoError(t, s.Save(path))

	reloaded, err := Load(path)
	require.NoError(t, err)
	// Save sorts+dedupes the list, like the permission lists.
	assert.Equal(t, []string{"docker *", "gh *", "git *"}, reloaded.ExcludedCommands())

	// Other sandbox keys + top-level keys survive untouched.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"enabled": true`)
	assert.Contains(t, string(data), `"denyRead"`)
	assert.Contains(t, string(data), `"model": "opus"`)
}

// TestSave_TwoSpaceIndentAndTrailingNewline — output uses 2-space
// indent (matches existing settings.json) and ends with a newline so
// line-based tooling and editors are happy.
func TestSave_TwoSpaceIndentAndTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Load(path)
	require.NoError(t, err)
	s.SetList(ListAllow, []string{"Bash(ls)"})
	require.NoError(t, s.Save(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	str := string(data)
	assert.Contains(t, str, "  \"permissions\": {")
	assert.Contains(t, str, "    \"allow\": [")
	assert.Contains(t, str, "      \"Bash(ls)\"")
	assert.True(t, str != "" && str[len(str)-1] == '\n', "must end with a newline")
}
