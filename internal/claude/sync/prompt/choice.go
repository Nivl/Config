// Package prompt is the interactive conflict resolution mechanism
// shared across settings/file/dir merges. Resolve consults — in
// order — the CLAUDE_MERGE_RESOLUTION env override, the decisions
// cache, and finally the user via stdin.
package prompt

// Choice is what Resolve returns.
type Choice int

const (
	// ChoiceKeepLocal means leave the local copy untouched.
	ChoiceKeepLocal Choice = iota
	// ChoiceTakeRemote means apply the remote value.
	ChoiceTakeRemote
	// ChoiceSkip means defer the decision — caller MUST NOT advance
	// last-sync-commit so the conflict re-surfaces next run.
	ChoiceSkip
)

// String returns a short stable string for this Choice, used in the
// "(env)" / "(remembered)" log lines.
func (c Choice) String() string {
	switch c {
	case ChoiceKeepLocal:
		return "keep-local"
	case ChoiceTakeRemote:
		return "take-remote"
	case ChoiceSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// Kind tags which decisions.json sub-object a remembered choice
// belongs to. KindSettings for settings.json merge-unit paths;
// KindFiles for relative file paths under .claude/.
type Kind string

const (
	// KindSettings selects the "settings" sub-object of decisions.json.
	KindSettings Kind = "settings"
	// KindFiles selects the "files" sub-object of decisions.json.
	KindFiles Kind = "files"
)
