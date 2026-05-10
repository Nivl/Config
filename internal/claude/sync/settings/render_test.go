package settings

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrAny(v any) *any { return &v } //nolint:modernize // suggested new(expr) rewrite doesn't apply to *any boxing

// TestSideRepr_Unchanged — base == side (after compact-JSON normalization).
func TestSideRepr_Unchanged(t *testing.T) {
	assert.Equal(t, "(unchanged)", sideRepr(ptrAny("opus"), ptrAny("opus")))
}

// TestSideRepr_Modify — both present, different.
func TestSideRepr_Modify(t *testing.T) {
	assert.Equal(t, `"opus" -> "haiku"`, sideRepr(ptrAny("opus"), ptrAny("haiku")))
}

// TestSideRepr_Deleted — base present, side absent.
func TestSideRepr_Deleted(t *testing.T) {
	assert.Equal(t, `"opus" -> (deleted)`, sideRepr(ptrAny("opus"), nil))
}

// TestSideRepr_Added — base absent, side present.
func TestSideRepr_Added(t *testing.T) {
	assert.Equal(t, `(added) "haiku"`, sideRepr(nil, ptrAny("haiku")))
}

// TestConflictRenderer_DiffModifyOnBoth — both sides modified to different values.
func TestConflictRenderer_DiffModifyOnBoth(t *testing.T) {
	r := newConflictRenderer(Decision{
		BaseValue:   ptrAny("opus"),
		LocalValue:  ptrAny("haiku"),
		RemoteValue: ptrAny("sonnet"),
	}, "abc1234")
	var buf bytes.Buffer
	r.Diff(&buf)
	assert.Equal(t, "  (diff) local: \"opus\" -> \"haiku\"  remote: \"opus\" -> \"sonnet\"\n", buf.String())
}

// TestConflictRenderer_DiffLocalDeletedRemoteModified — local deleted, remote modified.
func TestConflictRenderer_DiffLocalDeletedRemoteModified(t *testing.T) {
	r := newConflictRenderer(Decision{
		BaseValue:   ptrAny("opus"),
		LocalValue:  nil,
		RemoteValue: ptrAny("sonnet"),
	}, "abc1234")
	var buf bytes.Buffer
	r.Diff(&buf)
	assert.Equal(t, "  (diff) local: \"opus\" -> (deleted)  remote: \"opus\" -> \"sonnet\"\n", buf.String())
}

// TestConflictRenderer_DiffAddAdd — base absent, both sides added different values.
func TestConflictRenderer_DiffAddAdd(t *testing.T) {
	r := newConflictRenderer(Decision{
		BaseValue:   nil,
		LocalValue:  ptrAny("haiku"),
		RemoteValue: ptrAny("sonnet"),
	}, "none")
	var buf bytes.Buffer
	r.Diff(&buf)
	assert.Equal(t, "  (diff) local: (added) \"haiku\"  remote: (added) \"sonnet\"\n", buf.String())
}

// TestConflictRenderer_DiffOnlyRemoteChanged — local matches base, remote diverges.
func TestConflictRenderer_DiffOnlyRemoteChanged(t *testing.T) {
	r := newConflictRenderer(Decision{
		BaseValue:   ptrAny("opus"),
		LocalValue:  ptrAny("opus"),
		RemoteValue: ptrAny("sonnet"),
	}, "abc1234")
	var buf bytes.Buffer
	r.Diff(&buf)
	assert.Equal(t, "  (diff) local: (unchanged)  remote: \"opus\" -> \"sonnet\"\n", buf.String())
}

// TestRenderPath_SimpleIdentifier wraps a bare identifier with a dot.
func TestRenderPath_SimpleIdentifier(t *testing.T) {
	assert.Equal(t, ".model", renderPath([]string{"model"}))
}

// TestRenderPath_DotChain joins two simple identifiers with dots.
func TestRenderPath_DotChain(t *testing.T) {
	assert.Equal(t, ".permissions.allow", renderPath([]string{"permissions", "allow"}))
}

// TestRenderPath_SpecialCharsBracketed uses [".."] for keys with special chars.
// Bash's rule: a key matching ^[a-zA-Z_][a-zA-Z_0-9]*$ goes through .key;
// anything else goes through [".."].
func TestRenderPath_SpecialCharsBracketed(t *testing.T) {
	assert.Equal(t, `.enabledPlugins.["warp@x"]`, renderPath([]string{"enabledPlugins", "warp@x"}))
}

// TestRenderPath_DigitFirstBracketed verifies a leading digit forces bracket form.
func TestRenderPath_DigitFirstBracketed(t *testing.T) {
	assert.Equal(t, `.["1stkey"]`, renderPath([]string{"1stkey"}))
}

// TestRenderPath_EmptyKeyBracketed verifies the empty-string key is bracketed.
func TestRenderPath_EmptyKeyBracketed(t *testing.T) {
	assert.Equal(t, `.[""]`, renderPath([]string{""}))
}

// TestRenderPath_KeyWithQuoteEscaped verifies the bracket form JSON-escapes the key.
func TestRenderPath_KeyWithQuoteEscaped(t *testing.T) {
	assert.Equal(t, `.["a\"b"]`, renderPath([]string{`a"b`}))
}
