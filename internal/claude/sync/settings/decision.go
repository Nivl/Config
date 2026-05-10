// Package settings is the JSON 3-way merge engine for .claude/settings.json.
// The Decide function is pure: it takes three []byte inputs and produces
// a typed plan. All I/O (file reads, prompts, applying the plan) happens
// in Merge.
package settings

// Action is the terminal verdict for one decision unit.
type Action int

const (
	// ActionNoop means base, local, and remote agree (or the change is
	// local-only and harmless). Excluded from Decide's output.
	ActionNoop Action = iota
	// ActionTakeRemote means apply Decision.Value to local.
	ActionTakeRemote
	// ActionRemoteDelete means remove the path from local.
	ActionRemoteDelete
	// ActionKeepLocal means local already has the right value.
	ActionKeepLocal
	// ActionConflict means local and remote diverged differently from
	// base; the caller must Resolve via the prompt.
	ActionConflict
)

// Kind is the merge-unit categorization. KindValue is the default;
// KindSet is set when the decision unit is a primitive array treated
// as a set (every present value at the path is a primitive array).
type Kind int

const (
	// KindValue is the default — equality-based merge.
	KindValue Kind = iota
	// KindSet is the primitive-array set-merge classification.
	KindSet
)

// Decision is one node of the merge plan. Path is the JSON path
// segment list (e.g. ["permissions","allow"]). For Action=TakeRemote
// or RemoteDelete, the caller applies it. For Action=Conflict, the
// caller prompts using Base/Local/Remote pointers to render details.
//
// On a set-merge take-remote, Adds/Removes/Total carry the
// cardinalities the caller logs as "settings.json: <path> merged: +N
// -M (now K items)".
//
//nolint:govet // fieldalignment: logical grouping (path→action→kind→value→conflict pointers→cardinalities) takes priority
type Decision struct {
	// Path is the JSON path segment list, length 1 or 2 per the
	// dynamic-granularity rule.
	Path []string
	// Action is one of the five terminal verdicts.
	Action Action
	// Kind is KindSet for primitive-array set-merges, KindValue otherwise.
	Kind Kind

	// Value is set for Action=TakeRemote. It is the value to apply at
	// Path. For Kind=KindSet, it is the sorted merged set.
	Value any

	// BaseValue/LocalValue/RemoteValue are set for Action=Conflict.
	// Each pointer is nil if that side was absent at Path.
	BaseValue   *any
	LocalValue  *any
	RemoteValue *any

	// Adds/Removes/Total are populated for Kind=KindSet on take-remote.
	// Adds   = len(merged - localSet)
	// Removes= len(localSet - merged)
	// Total  = len(merged)
	Adds, Removes, Total int
}
