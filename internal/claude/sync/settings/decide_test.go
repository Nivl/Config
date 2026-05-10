package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptr is a tiny helper for *any pointer literals in test expectations.
func ptr[T any](v T) *any { var a any = v; return &a }

// TestDecide covers every branch of the decision algorithm, driving
// from JSON literal inputs.
func TestDecide(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		local  string
		remote string
		want   []Decision
	}{
		// All equal → noop (filtered out).
		{
			name:   "all_three_identical",
			base:   `{"model":"opus"}`,
			local:  `{"model":"opus"}`,
			remote: `{"model":"opus"}`,
			want:   nil,
		},
		// equal(B,L) AND R different and present → take-remote.
		{
			name:   "remote_modifies_scalar",
			base:   `{"model":"opus"}`,
			local:  `{"model":"opus"}`,
			remote: `{"model":"sonnet"}`,
			want: []Decision{{
				Path:   []string{"model"},
				Action: ActionTakeRemote,
				Value:  "sonnet",
			}},
		},
		// equal(B,L) and R absent → remote-delete.
		{
			name:   "remote_deletes_key",
			base:   `{"model":"opus"}`,
			local:  `{"model":"opus"}`,
			remote: `{}`,
			want: []Decision{{
				Path:   []string{"model"},
				Action: ActionRemoteDelete,
			}},
		},
		// equal(B,R), L different → keep-local.
		{
			name:   "local_modifies_remote_unchanged",
			base:   `{"model":"opus"}`,
			local:  `{"model":"haiku"}`,
			remote: `{"model":"opus"}`,
			want: []Decision{{
				Path:   []string{"model"},
				Action: ActionKeepLocal,
			}},
		},
		// equal(L,R) AND both present → noop.
		{
			name:   "local_and_remote_converged_to_same_new_value",
			base:   `{"model":"opus"}`,
			local:  `{"model":"sonnet"}`,
			remote: `{"model":"sonnet"}`,
			want:   nil,
		},
		// Both diverged differently → conflict.
		{
			name:   "modify_modify_conflict_on_scalar",
			base:   `{"model":"opus"}`,
			local:  `{"model":"haiku"}`,
			remote: `{"model":"sonnet"}`,
			want: []Decision{{
				Path:        []string{"model"},
				Action:      ActionConflict,
				BaseValue:   ptr[any]("opus"),
				LocalValue:  ptr[any]("haiku"),
				RemoteValue: ptr[any]("sonnet"),
			}},
		},
		// Granularity drills into permissions (all present occurrences
		// are objects).
		{
			name:   "permissions_drills_into_subkey",
			base:   `{"permissions":{"defaultMode":"ask"}}`,
			local:  `{"permissions":{"defaultMode":"ask"}}`,
			remote: `{"permissions":{"defaultMode":"acceptEdits"}}`,
			want: []Decision{{
				Path:   []string{"permissions", "defaultMode"},
				Action: ActionTakeRemote,
				Value:  "acceptEdits",
			}},
		},
		// Types mixed across sides (one side has the key as scalar)
		// → whole-value merge (no drill-down).
		{
			name:   "permissions_mixed_types_whole_value_merge",
			base:   `{}`,
			local:  `{}`,
			remote: `{"permissions":"some-scalar"}`,
			want: []Decision{{
				Path:   []string{"permissions"},
				Action: ActionTakeRemote,
				Value:  "some-scalar",
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide([]byte(tc.base), []byte(tc.local), []byte(tc.remote))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestDecide_SetMerge covers the set-merge branch.
func TestDecide_SetMerge(t *testing.T) {
	cases := []struct {
		name   string
		base   string
		local  string
		remote string
		want   []Decision
	}{
		// Pure reorder is noop.
		{
			name:   "pure_reorder_is_noop",
			base:   `{"x":["A","B","C"]}`,
			local:  `{"x":["C","A","B"]}`,
			remote: `{"x":["A","B","C"]}`,
			want:   nil,
		},
		// Remote-only add yields take-remote+kindset.
		{
			name:   "remote_only_add",
			base:   `{"x":["A","B"]}`,
			local:  `{"x":["A","B"]}`,
			remote: `{"x":["A","B","D"]}`,
			want: []Decision{{
				Path:    []string{"x"},
				Action:  ActionTakeRemote,
				Kind:    KindSet,
				Value:   []any{"A", "B", "D"},
				Adds:    1,
				Removes: 0,
				Total:   3,
			}},
		},
		// Local-only add preserved (noop).
		{
			name:   "local_only_add_preserved",
			base:   `{"x":["A","B"]}`,
			local:  `{"x":["A","B","C"]}`,
			remote: `{"x":["A","B"]}`,
			want:   nil,
		},
		// Both add disjoint → union.
		{
			name:   "both_add_disjoint",
			base:   `{"x":["A"]}`,
			local:  `{"x":["A","C"]}`,
			remote: `{"x":["A","D"]}`,
			want: []Decision{{
				Path:    []string{"x"},
				Action:  ActionTakeRemote,
				Kind:    KindSet,
				Value:   []any{"A", "C", "D"},
				Adds:    1, // D (relative to local set {A,C})
				Removes: 0,
				Total:   3,
			}},
		},
		// Asymmetric local-removes, remote-adds.
		{
			name:   "local_removes_remote_adds",
			base:   `{"x":["A","B"]}`,
			local:  `{"x":["A"]}`,         // removed B
			remote: `{"x":["A","B","D"]}`, // added D
			want: []Decision{{
				Path:    []string{"x"},
				Action:  ActionTakeRemote,
				Kind:    KindSet,
				Value:   []any{"A", "D"}, // B in l_removes is honored; D from remote-adds survives
				Adds:    1,               // D added relative to local set {A}
				Removes: 0,               // local set {A} is a subset of merged
				Total:   2,
			}},
		},
		// Array of objects falls back to equality merge.
		{
			name:   "array_of_objects_falls_back_to_conflict",
			base:   `{"hooks":[{"id":"a"}]}`,
			local:  `{"hooks":[{"id":"a"},{"id":"b"}]}`,
			remote: `{"hooks":[{"id":"a"},{"id":"c"}]}`,
			want: []Decision{{
				Path:        []string{"hooks"},
				Action:      ActionConflict,
				BaseValue:   ptr[any]([]any{map[string]any{"id": "a"}}),
				LocalValue:  ptr[any]([]any{map[string]any{"id": "a"}, map[string]any{"id": "b"}}),
				RemoteValue: ptr[any]([]any{map[string]any{"id": "a"}, map[string]any{"id": "c"}}),
			}},
		},
		// Empty array qualifies as primitive array; absent local →
		// as_set(L) = [] = merged → noop (no presence guard).
		{
			name:   "empty_array_qualifies_as_prim_array_with_absent_local_is_noop",
			base:   `{}`,
			local:  `{}`,
			remote: `{"x":[]}`,
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide([]byte(tc.base), []byte(tc.local), []byte(tc.remote))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestDecide_StableOrder verifies the output is sorted by path.
func TestDecide_StableOrder(t *testing.T) {
	base := `{}`
	local := `{}`
	remote := `{"z":"3","a":"1","m":"2"}`
	got, err := Decide([]byte(base), []byte(local), []byte(remote))
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"a"}, got[0].Path)
	assert.Equal(t, []string{"m"}, got[1].Path)
	assert.Equal(t, []string{"z"}, got[2].Path)
}

// TestDecide_AbsentOnAllSidesNotEmitted ensures keys absent everywhere
// produce no decision.
func TestDecide_AbsentOnAllSidesNotEmitted(t *testing.T) {
	got, err := Decide([]byte(`{}`), []byte(`{}`), []byte(`{}`))
	require.NoError(t, err)
	assert.Nil(t, got)
}
