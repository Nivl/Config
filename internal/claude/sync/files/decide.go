// Package files owns the file-and-directory merge engine for
// `~/.claude/`: a 13-row per-file truth table (MergeFile) and a
// three-source union walk over a curated directory (MergeDir).
// settings.json merge lives in the sibling settings/ package; the
// shared 3-tier resolver lives in prompt/.
package files

// Action is the 13-row truth table's output. Returned by Decide and
// consumed by MergeFile to dispatch the actual filesystem mutation.
type Action int

const (
	// ActionNoop means no filesystem mutation. Most truth-table rows
	// collapse to this when L/R/B converged.
	ActionNoop Action = iota
	// ActionCopyRemoteToLocal means overwrite L with R's bytes; if L
	// existed, it is backed up first.
	ActionCopyRemoteToLocal
	// ActionRemoveLocal means delete L (always backed up first). Used
	// only on the clean local-keep, remote-delete row.
	ActionRemoveLocal
	// ActionConflict means the truth table cannot resolve without user
	// input; MergeFile MUST invoke the 3-tier resolver.
	ActionConflict
)

// ConflictType narrows the conflict for the prompt header
// ("Conflict in <rel> (<type>)"). Empty unless Action ==
// ActionConflict.
type ConflictType string

const (
	// ConflictModifyModify — both sides diverged differently from base.
	ConflictModifyModify ConflictType = "modify-modify"
	// ConflictAddAddDiff — base absent, both sides added different content.
	ConflictAddAddDiff ConflictType = "add-add-diff"
	// ConflictModifyDelete — local modified, remote deleted.
	ConflictModifyDelete ConflictType = "modify-delete"
	// ConflictDeleteModify — local deleted, remote modified.
	ConflictDeleteModify ConflictType = "delete-modify"
)

// Presence holds (has_L, has_R, has_B): does the file exist locally,
// remotely, and at the base SHA respectively.
type Presence struct {
	// HasLocal is true if `$HOME/.claude/<rel>` exists.
	HasLocal bool
	// HasRemote is true if `<CONFIG_DIR>/.claude/<rel>` exists.
	HasRemote bool
	// HasBase is true if `git cat-file -e <sha>:.claude/<rel>` returns 0
	// AND the anchor SHA is non-empty.
	HasBase bool
}

// Equality holds (L_eq_B, R_eq_B, L_eq_R) — byte-equal comparisons
// between each pair of sides. Callers MUST set absent-side bits to
// false; Decide never reads them on absent sides, but a `true` on an
// absent-side bit would silently mis-route the truth table.
type Equality struct {
	// LocalEqBase is true iff has_L && has_B && bytes(L) == bytes(B).
	LocalEqBase bool
	// RemoteEqBase is true iff has_R && has_B && bytes(R) == bytes(B).
	RemoteEqBase bool
	// LocalEqRemote is true iff has_L && has_R && bytes(L) == bytes(R).
	LocalEqRemote bool
}

// Decision is what Decide returns. ConflictType is the empty string
// unless Action == ActionConflict.
//
//nolint:govet // fieldalignment: Action before ConflictType mirrors truth-table column order (verdict then detail).
type Decision struct {
	Action       Action
	ConflictType ConflictType
}

// Decide evaluates the 13-row truth table. Pure function — no I/O.
// Append-only evaluation order: row 1 fires first; if its predicate
// fails, row 2 is evaluated; etc.
//
// Row order matters in row 4 (L=R AND L≠B AND R≠B): both sides
// converged to the same new value, even though they technically
// diverged from base — the system trusts the convergence and
// collapses to ActionNoop rather than reporting a conflict.
//
// The two unreachable input patterns (all three absent; has_L=0,
// has_R=0, has_B=1) collapse to ActionNoop via the default branch.
// MergeDir wouldn't call MergeFile for a file absent from all three
// sources, and the both-cleanly-deleted case is effectively a no-op.
func Decide(p Presence, e Equality) Decision {
	switch {
	// Row 1: L=B AND R=B → noop (everyone agrees with base).
	case p.HasLocal && p.HasRemote && p.HasBase && e.LocalEqBase && e.RemoteEqBase:
		return Decision{Action: ActionNoop}
	// Row 2: L=B AND R≠B → copy-r-to-l (only remote changed).
	case p.HasLocal && p.HasRemote && p.HasBase && e.LocalEqBase && !e.RemoteEqBase:
		return Decision{Action: ActionCopyRemoteToLocal}
	// Row 3: L≠B AND R=B → noop (only local changed; respect it).
	case p.HasLocal && p.HasRemote && p.HasBase && !e.LocalEqBase && e.RemoteEqBase:
		return Decision{Action: ActionNoop}
	// Row 4: L≠B AND R≠B AND L=R → noop (both converged to same value).
	case p.HasLocal && p.HasRemote && p.HasBase && !e.LocalEqBase && !e.RemoteEqBase && e.LocalEqRemote:
		return Decision{Action: ActionNoop}
	// Row 5: L≠B AND R≠B AND L≠R → conflict modify-modify.
	case p.HasLocal && p.HasRemote && p.HasBase && !e.LocalEqBase && !e.RemoteEqBase && !e.LocalEqRemote:
		return Decision{Action: ActionConflict, ConflictType: ConflictModifyModify}
	// Row 6: has_B=0, L=R → noop (parallel add of same content).
	case p.HasLocal && p.HasRemote && !p.HasBase && e.LocalEqRemote:
		return Decision{Action: ActionNoop}
	// Row 7: has_B=0, L≠R → conflict add-add-diff.
	case p.HasLocal && p.HasRemote && !p.HasBase && !e.LocalEqRemote:
		return Decision{Action: ActionConflict, ConflictType: ConflictAddAddDiff}
	// Row 8: has_R=0, has_B=1, L=B → rm-l (clean remote-delete; local
	// unmodified from base, so safe to remove locally too).
	case p.HasLocal && !p.HasRemote && p.HasBase && e.LocalEqBase:
		return Decision{Action: ActionRemoveLocal}
	// Row 9: has_R=0, has_B=1, L≠B → conflict modify-delete.
	case p.HasLocal && !p.HasRemote && p.HasBase && !e.LocalEqBase:
		return Decision{Action: ActionConflict, ConflictType: ConflictModifyDelete}
	// Row 10: has_R=0, has_B=0 → noop (local-add; only on disk locally).
	case p.HasLocal && !p.HasRemote && !p.HasBase:
		return Decision{Action: ActionNoop}
	// Row 11: has_L=0, has_B=1, R=B → noop (clean local-delete).
	case !p.HasLocal && p.HasRemote && p.HasBase && e.RemoteEqBase:
		return Decision{Action: ActionNoop}
	// Row 12: has_L=0, has_B=1, R≠B → conflict delete-modify.
	case !p.HasLocal && p.HasRemote && p.HasBase && !e.RemoteEqBase:
		return Decision{Action: ActionConflict, ConflictType: ConflictDeleteModify}
	// Row 13: has_L=0, has_B=0 → copy-r-to-l (remote-add).
	case !p.HasLocal && p.HasRemote && !p.HasBase:
		return Decision{Action: ActionCopyRemoteToLocal}
	// Unreachable patterns (all three absent; B-only): collapse to noop.
	default:
		return Decision{Action: ActionNoop}
	}
}
