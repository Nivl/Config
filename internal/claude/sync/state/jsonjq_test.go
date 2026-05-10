package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarshalJq_PreservesHTMLChars — `<`, `>`, `&` must NOT be
// escaped to \u00xx; callers that depend on byte-stable output rely
// on the unescaped form.
func TestMarshalJq_PreservesHTMLChars(t *testing.T) {
	out, err := MarshalJq("Bash(diff a b > /tmp/c)")
	require.NoError(t, err)
	assert.Equal(t, `"Bash(diff a b > /tmp/c)"`, string(out))
}

// TestMarshalJq_NoTrailingNewline — Encoder.Encode appends '\n'; the
// helper trims it so callers can use the bytes inline.
func TestMarshalJq_NoTrailingNewline(t *testing.T) {
	out, err := MarshalJq("x")
	require.NoError(t, err)
	assert.Equal(t, `"x"`, string(out))
}

// TestMarshalJq_StringSliceCompact — array encoding has no whitespace.
func TestMarshalJq_StringSliceCompact(t *testing.T) {
	out, err := MarshalJq([]string{"permissions", "allow"})
	require.NoError(t, err)
	assert.Equal(t, `["permissions","allow"]`, string(out))
}
