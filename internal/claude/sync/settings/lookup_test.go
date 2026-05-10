package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLookup_PresentAtRoot returns the value when path is empty.
func TestLookup_PresentAtRoot(t *testing.T) {
	obj := map[string]any{"x": 1.0}
	pres := lookup(obj, nil)
	assert.True(t, pres.present)
	assert.Equal(t, obj, pres.value)
}

// TestLookup_PresentAtOneLevel returns the value at a top-level key.
func TestLookup_PresentAtOneLevel(t *testing.T) {
	obj := map[string]any{"model": "opus"}
	pres := lookup(obj, []string{"model"})
	assert.True(t, pres.present)
	assert.Equal(t, "opus", pres.value)
}

// TestLookup_PresentAtTwoLevels returns the value at a nested key.
func TestLookup_PresentAtTwoLevels(t *testing.T) {
	obj := map[string]any{"permissions": map[string]any{"allow": []any{"A"}}}
	pres := lookup(obj, []string{"permissions", "allow"})
	assert.True(t, pres.present)
	assert.Equal(t, []any{"A"}, pres.value)
}

// TestLookup_AbsentWhenIntermediateIsNotObject returns absent if any
// intermediate value is not a map.
func TestLookup_AbsentWhenIntermediateIsNotObject(t *testing.T) {
	obj := map[string]any{"x": "scalar"}
	pres := lookup(obj, []string{"x", "y"})
	assert.False(t, pres.present)
}

// TestLookup_AbsentWhenKeyMissing returns absent when the final key is missing.
func TestLookup_AbsentWhenKeyMissing(t *testing.T) {
	obj := map[string]any{"x": 1.0}
	pres := lookup(obj, []string{"y"})
	assert.False(t, pres.present)
}

// TestLookup_AbsentOnNonObjectRoot returns absent if path is non-empty
// and the root isn't an object.
func TestLookup_AbsentOnNonObjectRoot(t *testing.T) {
	pres := lookup("scalar", []string{"x"})
	assert.False(t, pres.present)
}

// TestEqualPresence_BothAbsentIsEqual — equal($x; $y) is true iff
// both absent or both present with equal values.
func TestEqualPresence_BothAbsentIsEqual(t *testing.T) {
	assert.True(t, equalPresence(presence{}, presence{}))
}

// TestEqualPresence_PresentNeAbsent absent and present are never equal.
func TestEqualPresence_PresentNeAbsent(t *testing.T) {
	assert.False(t, equalPresence(presence{present: true, value: 1.0}, presence{}))
}

// TestEqualPresence_DeepStructural compares maps and slices recursively.
func TestEqualPresence_DeepStructural(t *testing.T) {
	a := presence{present: true, value: map[string]any{"x": []any{"a", "b"}}}
	b := presence{present: true, value: map[string]any{"x": []any{"a", "b"}}}
	c := presence{present: true, value: map[string]any{"x": []any{"a", "c"}}}
	assert.True(t, equalPresence(a, b))
	assert.False(t, equalPresence(a, c))
}

// TestUnmarshalJSONForMerge unmarshals JSON into a generic any value.
// Object top-level is map[string]any. Numbers are float64.
func TestUnmarshalJSONForMerge(t *testing.T) {
	v, err := unmarshalForMerge([]byte(`{"x":1,"y":["a"]}`))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"x": float64(1), "y": []any{"a"}}, v)
}

// TestUnmarshalJSONForMerge_EmptyBytes treats empty input as "{}".
func TestUnmarshalJSONForMerge_EmptyBytes(t *testing.T) {
	v, err := unmarshalForMerge([]byte{})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, v)
}
