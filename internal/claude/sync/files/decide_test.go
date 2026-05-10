package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDecide_TruthTable exhaustively exercises the 13 documented
// rows of the file-merge truth table plus the two unreachable input
// patterns. Each case names its row to make a mismatch obvious in
// test output.
func TestDecide_TruthTable(t *testing.T) {
	cases := []struct {
		name string
		p    Presence
		e    Equality
		want Decision
	}{
		// Row 1: L=B AND R=B → noop
		{
			name: "row01_all_equal_to_base",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: true},
			e:    Equality{LocalEqBase: true, RemoteEqBase: true, LocalEqRemote: true},
			want: Decision{Action: ActionNoop},
		},
		// Row 2: L=B AND R≠B → copy-r-to-l
		{
			name: "row02_local_eq_base_remote_diverged",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: true},
			e:    Equality{LocalEqBase: true, RemoteEqBase: false, LocalEqRemote: false},
			want: Decision{Action: ActionCopyRemoteToLocal},
		},
		// Row 3: L≠B AND R=B → noop
		{
			name: "row03_remote_eq_base_local_diverged",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: true},
			e:    Equality{LocalEqBase: false, RemoteEqBase: true, LocalEqRemote: false},
			want: Decision{Action: ActionNoop},
		},
		// Row 4: L≠B AND R≠B AND L=R → noop (converged)
		{
			name: "row04_both_diverged_but_converged",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: true},
			e:    Equality{LocalEqBase: false, RemoteEqBase: false, LocalEqRemote: true},
			want: Decision{Action: ActionNoop},
		},
		// Row 5: L≠B AND R≠B AND L≠R → conflict modify-modify
		{
			name: "row05_modify_modify",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: true},
			e:    Equality{LocalEqBase: false, RemoteEqBase: false, LocalEqRemote: false},
			want: Decision{Action: ActionConflict, ConflictType: ConflictModifyModify},
		},
		// Row 6: has_B=0, L=R → noop
		{
			name: "row06_no_base_parallel_add_same_content",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: false},
			e:    Equality{LocalEqRemote: true},
			want: Decision{Action: ActionNoop},
		},
		// Row 7: has_B=0, L≠R → conflict add-add-diff
		{
			name: "row07_add_add_diff",
			p:    Presence{HasLocal: true, HasRemote: true, HasBase: false},
			e:    Equality{LocalEqRemote: false},
			want: Decision{Action: ActionConflict, ConflictType: ConflictAddAddDiff},
		},
		// Row 8: has_R=0, has_B=1, L=B → rm-l
		{
			name: "row08_clean_remote_delete",
			p:    Presence{HasLocal: true, HasRemote: false, HasBase: true},
			e:    Equality{LocalEqBase: true},
			want: Decision{Action: ActionRemoveLocal},
		},
		// Row 9: has_R=0, has_B=1, L≠B → conflict modify-delete
		{
			name: "row09_modify_delete",
			p:    Presence{HasLocal: true, HasRemote: false, HasBase: true},
			e:    Equality{LocalEqBase: false},
			want: Decision{Action: ActionConflict, ConflictType: ConflictModifyDelete},
		},
		// Row 10: has_R=0, has_B=0 → noop (local-add)
		{
			name: "row10_local_add",
			p:    Presence{HasLocal: true, HasRemote: false, HasBase: false},
			e:    Equality{},
			want: Decision{Action: ActionNoop},
		},
		// Row 11: has_L=0, has_B=1, R=B → noop (clean local-delete)
		{
			name: "row11_clean_local_delete",
			p:    Presence{HasLocal: false, HasRemote: true, HasBase: true},
			e:    Equality{RemoteEqBase: true},
			want: Decision{Action: ActionNoop},
		},
		// Row 12: has_L=0, has_B=1, R≠B → conflict delete-modify
		{
			name: "row12_delete_modify",
			p:    Presence{HasLocal: false, HasRemote: true, HasBase: true},
			e:    Equality{RemoteEqBase: false},
			want: Decision{Action: ActionConflict, ConflictType: ConflictDeleteModify},
		},
		// Row 13: has_L=0, has_B=0 → copy-r-to-l (remote-add)
		{
			name: "row13_remote_add",
			p:    Presence{HasLocal: false, HasRemote: true, HasBase: false},
			e:    Equality{},
			want: Decision{Action: ActionCopyRemoteToLocal},
		},
		// Unreachable: (0,0,0). Defensive default → noop.
		{
			name: "unreachable_all_absent",
			p:    Presence{},
			e:    Equality{},
			want: Decision{Action: ActionNoop},
		},
		// Unreachable: (0,0,1). Both cleanly deleted; defensive default → noop.
		{
			name: "unreachable_both_deleted_base_present",
			p:    Presence{HasLocal: false, HasRemote: false, HasBase: true},
			e:    Equality{},
			want: Decision{Action: ActionNoop},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.p, tc.e)
			assert.Equal(t, tc.want, got)
		})
	}
}
