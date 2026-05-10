package settings

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApply_TakeRemoteAtTopLevel writes a top-level key.
func TestApply_TakeRemoteAtTopLevel(t *testing.T) {
	local := []byte(`{"model":"opus"}`)
	out, err := Apply(local, []Decision{{
		Path: []string{"model"}, Action: ActionTakeRemote, Value: "sonnet",
	}})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "sonnet", got["model"])
}

// TestApply_TakeRemoteAtNestedPath writes a nested key, creating the
// parent object if absent.
func TestApply_TakeRemoteAtNestedPath(t *testing.T) {
	local := []byte(`{}`)
	out, err := Apply(local, []Decision{{
		Path:   []string{"permissions", "defaultMode"},
		Action: ActionTakeRemote,
		Value:  "ask",
	}})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, map[string]any{"defaultMode": "ask"}, got["permissions"])
}

// TestApply_RemoteDeleteRemovesKey removes a top-level key.
func TestApply_RemoteDeleteRemovesKey(t *testing.T) {
	local := []byte(`{"model":"opus","theme":"dark"}`)
	out, err := Apply(local, []Decision{{
		Path: []string{"model"}, Action: ActionRemoteDelete,
	}})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.NotContains(t, got, "model")
	assert.Contains(t, got, "theme")
}

// TestApply_NoopAndKeepLocalDoNotMutate skips ActionNoop and ActionKeepLocal.
func TestApply_NoopAndKeepLocalDoNotMutate(t *testing.T) {
	local := []byte(`{"x":1}`)
	out, err := Apply(local, []Decision{
		{Path: []string{"x"}, Action: ActionNoop},
		{Path: []string{"y"}, Action: ActionKeepLocal, Value: "ignored"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"x":1}`, string(out))
}

// TestApply_PanicsOnUnresolvedConflict — ActionConflict must be resolved
// by the caller before Apply.
func TestApply_PanicsOnUnresolvedConflict(t *testing.T) {
	local := []byte(`{}`)
	assert.Panics(t, func() {
		_, _ = Apply(local, []Decision{{
			Path: []string{"x"}, Action: ActionConflict,
		}})
	})
}

// TestApply_OutputIsPrettyPrintedWithTrailingNewline — output is
// 2-space-indented JSON with a trailing newline, so settings.json
// doesn't collapse to one line.
func TestApply_OutputIsPrettyPrintedWithTrailingNewline(t *testing.T) {
	local := []byte(`{"model":"opus"}`)
	out, err := Apply(local, []Decision{{
		Path: []string{"model"}, Action: ActionTakeRemote, Value: "sonnet",
	}})
	require.NoError(t, err)
	//nolint:testifylint // intentional byte-exact compare; JSONEq would defeat the purpose
	assert.Equal(t, "{\n  \"model\": \"sonnet\"\n}\n", string(out))
}

// TestApply_DoesNotHTMLEscapeStrings — Bash(... > foo),
// Bash(grep <pat>), and Bash(a && b) are common permission entries.
// Apply must preserve the raw chars (no `&gt;`, `&lt;`, `&amp;`) so
// they round-trip cleanly through the canonicalize.jq pre-commit
// hook.
func TestApply_DoesNotHTMLEscapeStrings(t *testing.T) {
	local := []byte(`{"permissions":{"allow":[]}}`)
	out, err := Apply(local, []Decision{{
		Path:   []string{"permissions", "allow"},
		Action: ActionTakeRemote,
		Value:  []any{"Bash(diff a b > /tmp/c)", "Bash(grep <pat> file)", "Bash(a && b)"},
	}})
	require.NoError(t, err)
	got := string(out)
	assert.Contains(t, got, "Bash(diff a b > /tmp/c)")
	assert.Contains(t, got, "Bash(grep <pat> file)")
	assert.Contains(t, got, "Bash(a && b)")
	// No \uXXXX escape sequences for the HTML-special chars. Use
	// concatenation so the backslash isn't interpreted in source.
	assert.NotContains(t, got, "\\u003e")
	assert.NotContains(t, got, "\\u003c")
	assert.NotContains(t, got, "\\u0026")
}
